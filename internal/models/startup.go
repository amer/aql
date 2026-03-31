package models

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/amer/aql/internal/domain"
)

// LoadOrDefault loads the saved model and cached model list from disk.
// Falls back through: saved model → first cached model → default model.
// Returns the resolved model ID and any cached models.
func LoadOrDefault(workDir string) (modelID string, cached []domain.ModelInfo) {
	savedModel, err := LoadModel(workDir)
	if err != nil {
		slog.Warn("failed to load saved model", "error", err)
	}

	cached, _ = LoadModelCache(workDir)
	if savedModel == "" && len(cached) > 0 {
		savedModel = cached[0].ID
		slog.Info("auto-selected model from cache", "model", savedModel)
	}
	if savedModel == "" {
		savedModel = string(ResolveModel(""))
	}

	return savedModel, cached
}

// ModelToTier converts a ModelInfo to a display-friendly tier description.
func ModelToTier(m domain.ModelInfo) (label, modelID, description string) {
	return m.DisplayName, m.ID, fmt.Sprintf("%dk context", m.MaxInputTokens/1000)
}

// ProbeAndUpdate probes usable models in the background, updates the cache,
// and calls onUpdate with the results. Designed to run in a goroutine.
func ProbeAndUpdate(ctx context.Context, apiKey string, isOAuth bool, workDir string, onUpdate func([]domain.ModelInfo)) {
	var usableModels []domain.ModelInfo
	var probeErr error
	if isOAuth {
		usableModels, probeErr = ProbeUsableModelsWithOAuthKey(ctx, apiKey)
	} else {
		usableModels, probeErr = ProbeUsableModelsWithAPIKey(ctx, apiKey)
	}
	if probeErr != nil {
		slog.Warn("background model probe failed", "error", probeErr)
		return
	}
	if len(usableModels) == 0 {
		return
	}

	if err := SaveModelCache(workDir, usableModels); err != nil {
		slog.Warn("failed to save model cache", "error", err)
	}

	onUpdate(usableModels)
}
