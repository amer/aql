package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
)

// ModelInfo holds information about an available model from the API.
type ModelInfo struct {
	ID             string
	DisplayName    string
	MaxInputTokens int64
	CreatedAt      time.Time
}

// FetchModels lists available models from the Anthropic API.
func FetchModels(ctx context.Context) ([]ModelInfo, error) {
	client := anthropic.NewClient()
	return fetchModelsWithClient(ctx, client)
}

// FetchModelsWithBaseURL lists available models using a custom API base URL.
func FetchModelsWithBaseURL(ctx context.Context, baseURL string) ([]ModelInfo, error) {
	client := anthropic.NewClient(
		option.WithBaseURL(baseURL),
		option.WithAPIKey("test-key"),
	)
	return fetchModelsWithClient(ctx, client)
}

func fetchModelsWithClient(ctx context.Context, client anthropic.Client) ([]ModelInfo, error) {
	slog.Debug("fetching available models from API")

	pager := client.Models.ListAutoPaging(ctx, anthropic.ModelListParams{
		Limit: param.NewOpt[int64](100),
	})

	var models []ModelInfo
	for pager.Next() {
		m := pager.Current()
		models = append(models, ModelInfo{
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

// ResolveModel maps a model string to an anthropic.Model.
// Supports shortcuts ("haiku", "sonnet", "opus") and full model IDs.
// Defaults to Haiku if empty.
func ResolveModel(model string) anthropic.Model {
	switch model {
	case "", "haiku":
		return anthropic.ModelClaudeHaiku4_5
	case "sonnet":
		return anthropic.ModelClaudeSonnet4_5
	case "opus":
		return anthropic.ModelClaudeOpus4_5
	default:
		return anthropic.Model(model)
	}
}

const modelFileName = ".aql_model"

// SaveModel persists the selected model ID to a file in the given directory.
func SaveModel(dir string, model string) error {
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
