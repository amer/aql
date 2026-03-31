package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
// request to determine which models the API key can actually use. Models that
// return 400/403 are filtered out.
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
// This unlocks Opus/Sonnet for OAuth Console users.
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
// This avoids wasting time probing legacy or irrelevant models.
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

	// Filter to relevant models before probing
	var candidates []domain.ModelInfo
	for _, m := range models {
		if isRelevantModel(m.ID) {
			candidates = append(candidates, m)
		}
	}

	slog.Debug("probing model access", "totalModels", len(models), "candidates", len(candidates), "billing", withBilling)

	// Probe in parallel
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

// probeModel sends a minimal request to check if a model is accessible.
// When withBilling is true, includes the Claude Code billing header for OAuth access.
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
			{Text: billingHeader},
		}
		params.Thinking = anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
		}
		params.OutputConfig = anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortMedium,
		}
		params.MaxTokens = probeMaxTokensBilling
		reqOpts = append(reqOpts, option.WithHeader("anthropic-beta", claudeCodeBetas))
	}

	_, err := client.Messages.New(ctx, params, reqOpts...)
	return err == nil
}

// ResolveModel maps a model string to an anthropic.Model.
// Supports shortcuts ("haiku", "sonnet", "opus") and full model IDs.
// Defaults to Sonnet if empty. Rejects obviously invalid values
// (slash commands) by falling back to the default.
func ResolveModel(model string) anthropic.Model {
	switch model {
	case "", "sonnet":
		return anthropic.ModelClaudeSonnet4_6
	case "opus":
		return anthropic.ModelClaudeOpus4_6
	case "haiku":
		return anthropic.ModelClaudeHaiku4_5
	default:
		if err := ValidateModelID(model); err != nil {
			slog.Warn("invalid model ID, using default", "model", model, "error", err)
			return anthropic.ModelClaudeSonnet4_6
		}
		return anthropic.Model(model)
	}
}

const modelFileName = ".aql_model"

// ValidateModelID checks that a model ID looks valid (not empty, not a slash command).
func ValidateModelID(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return fmt.Errorf("model ID must not be empty")
	}
	if strings.HasPrefix(trimmed, "/") {
		return fmt.Errorf("model ID %q looks like a slash command, not a model", id)
	}
	return nil
}

// SaveModel persists the selected model ID to a file in the given directory.
// Returns an error if the model ID is invalid (e.g., a slash command).
func SaveModel(dir string, model string) error {
	if err := ValidateModelID(model); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, modelFileName), []byte(model), 0644)
}

// LoadModel reads the persisted model ID from the given directory.
// Returns empty string if no file exists (caller should use default).
func LoadModel(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, modelFileName))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
