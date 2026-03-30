package agent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/amer/aql/internal/agent"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveModelDefault(t *testing.T) {
	model := agent.ResolveModel("")
	assert.Equal(t, anthropic.ModelClaudeHaiku4_5, model)
}

func TestResolveModelExplicit(t *testing.T) {
	model := agent.ResolveModel("claude-sonnet-4-5")
	assert.Equal(t, anthropic.Model("claude-sonnet-4-5"), model)
}

func TestResolveModelShortcuts(t *testing.T) {
	assert.Equal(t, anthropic.ModelClaudeHaiku4_5, agent.ResolveModel("haiku"))
	assert.Equal(t, anthropic.ModelClaudeSonnet4_5, agent.ResolveModel("sonnet"))
	assert.Equal(t, anthropic.ModelClaudeOpus4_5, agent.ResolveModel("opus"))
}

func TestConfigModel(t *testing.T) {
	cfg := agent.Config{
		Name:  "test",
		Model: "haiku",
	}
	assert.Equal(t, "haiku", cfg.Model)
}

func TestFetchModelsFromFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/models_list.json")
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	models, err := agent.FetchModelsWithBaseURL(ctx, server.URL)
	require.NoError(t, err)
	assert.True(t, len(models) > 0, "should return at least one model")

	// Check that each model has an ID, display name, and context window
	for _, m := range models {
		assert.NotEmpty(t, m.ID)
		assert.NotEmpty(t, m.DisplayName)
		assert.True(t, m.MaxInputTokens > 0, "model %s should have context window size", m.ID)
	}

	// Should include 1M context models
	var foundOpus46 bool
	for _, m := range models {
		if m.ID == "claude-opus-4-6-20260301" {
			foundOpus46 = true
			assert.Equal(t, int64(1000000), m.MaxInputTokens)
		}
	}
	assert.True(t, foundOpus46, "should include Opus 4.6 with 1M context")

	// Should be sorted newest first
	assert.Contains(t, models[0].ID, "4-6", "newest models should be first")
}

func TestFetchModelsLive(t *testing.T) {
	if os.Getenv("AQL_LIVE_TEST") != "1" {
		t.Skip("set AQL_LIVE_TEST=1 to run live API tests")
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	models, err := agent.FetchModels(ctx)
	require.NoError(t, err)
	assert.True(t, len(models) > 0, "should return at least one model from API")
	t.Logf("fetched %d models from API:", len(models))
	for _, m := range models {
		t.Logf("  %s — %s — %dk ctx — %s", m.ID, m.DisplayName, m.MaxInputTokens/1000, m.CreatedAt.Format("2006-01-02"))
	}
}

func TestSaveAndLoadModel(t *testing.T) {
	dir := t.TempDir()

	err := agent.SaveModel(dir, "claude-sonnet-4-20250514")
	assert.NoError(t, err)

	loaded, err := agent.LoadModel(dir)
	assert.NoError(t, err)
	assert.Equal(t, "claude-sonnet-4-20250514", loaded)
}

func TestLoadModelDefault(t *testing.T) {
	dir := t.TempDir()

	loaded, err := agent.LoadModel(dir)
	assert.NoError(t, err)
	assert.Equal(t, "", loaded)
}

func TestSaveModelOverwrites(t *testing.T) {
	dir := t.TempDir()

	agent.SaveModel(dir, "claude-haiku-4-5-20241022")
	agent.SaveModel(dir, "claude-opus-4-20250415")

	loaded, _ := agent.LoadModel(dir)
	assert.Equal(t, "claude-opus-4-20250415", loaded)
}
