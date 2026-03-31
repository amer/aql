package agent

import (
	"context"
	"time"

	"github.com/amer/aql/internal/domain"
	"github.com/amer/aql/internal/models"
	"github.com/anthropics/anthropic-sdk-go"
)

// Forwarding functions — canonical implementations live in the models package.

func FetchModels(ctx context.Context) ([]domain.ModelInfo, error) {
	return models.FetchModels(ctx)
}

func FetchModelsWithBaseURL(ctx context.Context, baseURL string) ([]domain.ModelInfo, error) {
	return models.FetchModelsWithBaseURL(ctx, baseURL)
}

func ProbeUsableModels(ctx context.Context) ([]domain.ModelInfo, error) {
	return models.ProbeUsableModels(ctx)
}

func ProbeUsableModelsWithBaseURL(ctx context.Context, baseURL string) ([]domain.ModelInfo, error) {
	return models.ProbeUsableModelsWithBaseURL(ctx, baseURL)
}

func ProbeUsableModelsWithAPIKey(ctx context.Context, apiKey string) ([]domain.ModelInfo, error) {
	return models.ProbeUsableModelsWithAPIKey(ctx, apiKey)
}

func ProbeUsableModelsWithBilling(ctx context.Context, baseURL string, apiKey string) ([]domain.ModelInfo, error) {
	return models.ProbeUsableModelsWithBilling(ctx, baseURL, apiKey)
}

func ProbeUsableModelsWithOAuthKey(ctx context.Context, apiKey string) ([]domain.ModelInfo, error) {
	return models.ProbeUsableModelsWithOAuthKey(ctx, apiKey)
}

func ResolveModel(model string) anthropic.Model {
	return models.ResolveModel(model)
}

func ValidateModelID(id string) error {
	return models.ValidateModelID(id)
}

func SaveModel(dir string, model string) error {
	return models.SaveModel(dir, model)
}

func LoadModel(dir string) (string, error) {
	return models.LoadModel(dir)
}

func SaveModelCache(dir string, ms []domain.ModelInfo) error {
	return models.SaveModelCache(dir, ms)
}

func SaveModelCacheWithTTL(dir string, ms []domain.ModelInfo, ttl time.Duration) error {
	return models.SaveModelCacheWithTTL(dir, ms, ttl)
}

func LoadModelCache(dir string) ([]domain.ModelInfo, error) {
	return models.LoadModelCache(dir)
}
