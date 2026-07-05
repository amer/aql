package models

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - LoadOrDefault() — startup model resolution (saved → cached → default)
//   - ProbeAndUpdate() — background model probe + cache update
//
// MUST NOT GO HERE:
//   - Direct Anthropic SDK calls (probe.go)
//   - Model persistence logic (persist.go)
//   - TUI imports
//
// Q: Should I change the model fallback order?
// A: Update LoadOrDefault() here.
// ──────────────────────────────────────────────────────────────────

import (
	"context"
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

	cached, err = LoadModelCache(workDir)
	if err != nil {
		// A missing cache is normal on first run; log at debug so a real
		// read/parse failure is still traceable without startup noise.
		slog.Debug("failed to load model cache", "error", err)
	}
	if savedModel == "" && len(cached) > 0 {
		savedModel = cached[0].ID
		slog.Info("auto-selected model from cache", "model", savedModel)
	}
	if savedModel == "" {
		savedModel = string(ResolveModel(""))
	}

	return savedModel, cached
}

// ProbeAndUpdate probes usable models in the background, updates the cache,
// and calls onUpdate with the results. Designed to run in a goroutine.
func ProbeAndUpdate(ctx context.Context, cfg ClientConfig, workDir string, onUpdate func([]domain.ModelInfo)) {
	usableModels, probeErr := ProbeUsableModels(ctx, cfg)
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
