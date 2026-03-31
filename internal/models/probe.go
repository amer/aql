package models

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - ClientConfig, FetchModels() — lists models from API
//   - ProbeUsableModels() — tests which models the key can use
//   - probeModel — single model probe
//   - isRelevantModel filter, relevantModelPrefixes
//
// MUST NOT GO HERE:
//   - Model persistence (persist.go)
//   - Model resolution/shortcuts (resolve.go)
//   - TUI imports
//
// Q: Should I add a new model family to probe?
// A: Add its prefix to relevantModelPrefixes.
//
// Q: How are models probed?
// A: Parallel minimal API calls — each model gets a one-token
//    request to test access.
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/amer/aql/internal/domain"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
)

const (
	// modelListLimit is the pagination limit for the models list endpoint.
	modelListLimit = 100

	// probeTimeout is the timeout for a single model probe request.
	probeTimeout = 5 * time.Second

	// probeMaxTokens is the minimal max_tokens for a non-billing probe.
	probeMaxTokens = 1

	// probeMaxTokensBilling is the max_tokens for an OAuth billing probe.
	probeMaxTokensBilling = 1024

	// BillingHeader re-exports domain.BillingHeader for backwards compatibility.
	BillingHeader = domain.BillingHeader

	// ClaudeCodeBetas re-exports domain.ClaudeCodeBetas for backwards compatibility.
	ClaudeCodeBetas = domain.ClaudeCodeBetas
)

// ClientConfig holds the parameters needed to construct an Anthropic client
// and configure probe behavior. Pass this to FetchModels and ProbeUsableModels
// instead of using variant-specific functions.
type ClientConfig struct {
	APIKey      string
	BaseURL     string
	WithBilling bool
	HTTPClient  *http.Client
}

// buildClient constructs an anthropic.Client from the config.
func (c ClientConfig) buildClient() anthropic.Client {
	var opts []option.RequestOption
	if c.APIKey != "" {
		opts = append(opts, option.WithAPIKey(c.APIKey))
	}
	if c.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(c.BaseURL))
	}
	if c.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(c.HTTPClient))
	}
	return anthropic.NewClient(opts...)
}

// FetchModels lists available models from the Anthropic API.
func FetchModels(ctx context.Context, cfg ClientConfig) ([]domain.ModelInfo, error) {
	client := cfg.buildClient()
	return fetchModelsWithClient(ctx, client)
}

func fetchModelsWithClient(ctx context.Context, client anthropic.Client) ([]domain.ModelInfo, error) {
	slog.Debug("fetching available models from API")

	pager := client.Models.ListAutoPaging(ctx, anthropic.ModelListParams{
		Limit: param.NewOpt[int64](modelListLimit),
	})

	var models []domain.ModelInfo
	for pager.Next() {
		m := pager.Current()
		models = append(models, domain.ModelInfo{
			ID:             m.ID,
			DisplayName:    m.DisplayName,
			MaxInputTokens: m.MaxInputTokens,
			CreatedAt:      m.CreatedAt,
		})
	}
	if err := pager.Err(); err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}

	// Sort by newest first
	sort.Slice(models, func(i, j int) bool {
		return models[i].CreatedAt.After(models[j].CreatedAt)
	})

	slog.Debug("fetched models", "count", len(models))
	return models, nil
}

// ProbeUsableModels fetches the model list and probes each one with a minimal
// request to determine which models the API key can actually use.
func ProbeUsableModels(ctx context.Context, cfg ClientConfig) ([]domain.ModelInfo, error) {
	client := cfg.buildClient()
	return probeUsableModelsWithClient(ctx, client, cfg.WithBilling)
}

// relevantModelPrefixes filters the model list to only models we care about probing.
var relevantModelPrefixes = []string{
	"claude-opus",
	"claude-sonnet",
	"claude-haiku",
}

func isRelevantModel(id string) bool {
	for _, prefix := range relevantModelPrefixes {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}
	return false
}

func probeUsableModelsWithClient(ctx context.Context, client anthropic.Client, withBilling bool) ([]domain.ModelInfo, error) {
	models, err := fetchModelsWithClient(ctx, client)
	if err != nil {
		return nil, err
	}

	var candidates []domain.ModelInfo
	for _, m := range models {
		if isRelevantModel(m.ID) {
			candidates = append(candidates, m)
		}
	}

	slog.Debug("probing model access", "totalModels", len(models), "candidates", len(candidates), "billing", withBilling)

	type result struct {
		model domain.ModelInfo
		ok    bool
	}
	results := make([]result, len(candidates))
	var wg sync.WaitGroup
	for i, m := range candidates {
		wg.Add(1)
		go func(idx int, model domain.ModelInfo) {
			defer wg.Done()
			ok := probeModel(ctx, client, model.ID, withBilling)
			results[idx] = result{model: model, ok: ok}
			if ok {
				slog.Debug("model accessible", "model", model.ID)
			} else {
				slog.Debug("model not accessible", "model", model.ID)
			}
		}(i, m)
	}
	wg.Wait()

	var usable []domain.ModelInfo
	for _, r := range results {
		if r.ok {
			usable = append(usable, r.model)
		}
	}

	slog.Info("model probe complete", "total", len(models), "candidates", len(candidates), "usable", len(usable))
	return usable, nil
}

func probeModel(ctx context.Context, client anthropic.Client, modelID string, withBilling bool) bool {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(modelID),
		MaxTokens: probeMaxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(".")),
		},
	}

	var reqOpts []option.RequestOption

	if withBilling {
		params.System = []anthropic.TextBlockParam{
			{Text: BillingHeader},
		}
		params.Thinking = anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
		}
		params.OutputConfig = anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortMedium,
		}
		params.MaxTokens = probeMaxTokensBilling
		reqOpts = append(reqOpts, option.WithHeader("anthropic-beta", ClaudeCodeBetas))
	}

	_, err := client.Messages.New(ctx, params, reqOpts...)
	return err == nil
}
