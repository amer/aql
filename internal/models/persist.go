package models

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amer/aql/internal/domain"
)

const (
	modelFileName  = ".aql_model"
	modelCacheFile = ".aql_models_cache.json"
	modelCacheTTL  = 1 * time.Hour
)

// SaveModel persists the selected model ID to a file in the given directory.
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

type modelCache struct {
	Models    []domain.ModelInfo `json:"models"`
	ExpiresAt time.Time          `json:"expires_at"`
}

// SaveModelCache persists the probed model list with the default TTL.
func SaveModelCache(dir string, models []domain.ModelInfo) error {
	return SaveModelCacheWithTTL(dir, models, modelCacheTTL)
}

// SaveModelCacheWithTTL persists the probed model list with a custom TTL.
func SaveModelCacheWithTTL(dir string, models []domain.ModelInfo, ttl time.Duration) error {
	cache := modelCache{
		Models:    models,
		ExpiresAt: time.Now().Add(ttl),
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, modelCacheFile)
	slog.Debug("saving model cache", "path", path, "models", len(models), "ttl", ttl)
	return os.WriteFile(path, data, 0644)
}

// LoadModelCache loads the cached model list. Returns nil if no cache exists
// or if the cache has expired.
func LoadModelCache(dir string) ([]domain.ModelInfo, error) {
	path := filepath.Join(dir, modelCacheFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var cache modelCache
	if err := json.Unmarshal(data, &cache); err != nil {
		slog.Warn("corrupt model cache, ignoring", "error", err)
		return nil, nil
	}

	if time.Now().After(cache.ExpiresAt) {
		slog.Debug("model cache expired", "expiredAt", cache.ExpiresAt)
		return nil, nil
	}

	slog.Debug("loaded model cache", "models", len(cache.Models), "expiresAt", cache.ExpiresAt)
	return cache.Models, nil
}
