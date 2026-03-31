package models

import (
	"context"
	"fmt"
	"log/slog"
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

	// BillingHeader is the Claude Code billing header that enables access to
	// Opus/Sonnet models via OAuth Console login.
	BillingHeader = "x-anthropic-billing-header: cc_version=2.1.87.7b6; cc_entrypoint=cli; cch=22c94;"

	// ClaudeCodeBetas are the beta feature flags required for Claude Code billing.
	ClaudeCodeBetas = "claude-code-20250219,interleaved-thinking-2025-05-14,effort-2025-11-24"
)

// FetchModels lists available models from the Anthropic API.
func FetchModels(ctx context.Context) ([]domain.ModelInfo, error) {
	client := anthropic.NewClient()
	return fetchModelsWithClient(ctx, client)
}

// FetchModelsWithBaseURL lists available models using a custom API base URL.
func FetchModelsWithBaseURL(ctx context.Context, baseURL string) ([]domain.ModelInfo, error) {
	client := anthropic.NewClient(
		option.WithBaseURL(baseURL),
		option.WithAPIKey("test-key"),
	)
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
func ProbeUsableModels(ctx context.Context) ([]domain.ModelInfo, error) {
	client := anthropic.NewClient()
	return probeUsableModelsWithClient(ctx, client, false)
}

// ProbeUsableModelsWithBaseURL is like ProbeUsableModels but uses a custom base URL.
func ProbeUsableModelsWithBaseURL(ctx context.Context, baseURL string) ([]domain.ModelInfo, error) {
	client := anthropic.NewClient(
		option.WithBaseURL(baseURL),
		option.WithAPIKey("test-key"),
	)
	return probeUsableModelsWithClient(ctx, client, false)
}

// ProbeUsableModelsWithAPIKey probes models using a specific API key.
func ProbeUsableModelsWithAPIKey(ctx context.Context, apiKey string) ([]domain.ModelInfo, error) {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return probeUsableModelsWithClient(ctx, client, false)
}

// ProbeUsableModelsWithBilling probes models with the Claude Code billing header.
func ProbeUsableModelsWithBilling(ctx context.Context, baseURL string, apiKey string) ([]domain.ModelInfo, error) {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := anthropic.NewClient(opts...)
	return probeUsableModelsWithClient(ctx, client, true)
}

// ProbeUsableModelsWithOAuthKey probes models using an OAuth API key with billing header.
func ProbeUsableModelsWithOAuthKey(ctx context.Context, apiKey string) ([]domain.ModelInfo, error) {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return probeUsableModelsWithClient(ctx, client, true)
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
