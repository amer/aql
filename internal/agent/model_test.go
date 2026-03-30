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
	assert.Equal(t, anthropic.ModelClaudeSonnet4_6, model)
}

func TestResolveModelExplicit(t *testing.T) {
	model := agent.ResolveModel("claude-sonnet-4-5")
	assert.Equal(t, anthropic.Model("claude-sonnet-4-5"), model)
}

func TestResolveModelShortcuts(t *testing.T) {
	assert.Equal(t, anthropic.ModelClaudeHaiku4_5, agent.ResolveModel("haiku"))
	assert.Equal(t, anthropic.ModelClaudeSonnet4_6, agent.ResolveModel("sonnet"))
	assert.Equal(t, anthropic.ModelClaudeOpus4_6, agent.ResolveModel("opus"))
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

func TestProbeUsableModels(t *testing.T) {
	// Server that allows haiku but rejects opus
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			// Return model list
			fixture, _ := os.ReadFile("testdata/models_list.json")
			w.Header().Set("Content-Type", "application/json")
			w.Write(fixture)
			return
		}

		// For messages endpoint, parse body to check model
		body := make([]byte, 4096)
		n, _ := r.Body.Read(body)
		bodyStr := string(body[:n])

		if contains(bodyStr, "haiku") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"model":"claude-haiku-4-5","stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":1}}`))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"Error"}}`))
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usable, err := agent.ProbeUsableModelsWithBaseURL(ctx, server.URL)
	require.NoError(t, err)

	// Should only contain haiku models (the ones that return 200)
	assert.True(t, len(usable) > 0, "should find at least one usable model")
	for _, m := range usable {
		assert.Contains(t, m.ID, "haiku", "only haiku models should be usable")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestProbeUsableModelsWithBilling(t *testing.T) {
	// Server that allows all models when billing header is present,
	// only haiku without it (simulating the real API behavior)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			fixture, _ := os.ReadFile("testdata/models_list.json")
			w.Header().Set("Content-Type", "application/json")
			w.Write(fixture)
			return
		}

		body := make([]byte, 8192)
		n, _ := r.Body.Read(body)
		bodyStr := string(body[:n])

		hasBilling := contains(bodyStr, "x-anthropic-billing-header")

		if hasBilling || contains(bodyStr, "haiku") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"model":"test","stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":1}}`))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"Error"}}`))
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Without billing: only haiku
	withoutBilling, err := agent.ProbeUsableModelsWithBaseURL(ctx, server.URL)
	require.NoError(t, err)
	for _, m := range withoutBilling {
		assert.Contains(t, m.ID, "haiku", "without billing, only haiku should work")
	}

	// With billing: all models
	withBilling, err := agent.ProbeUsableModelsWithBilling(ctx, server.URL, "test-key")
	require.NoError(t, err)
	assert.True(t, len(withBilling) > len(withoutBilling),
		"billing header should unlock more models: got %d vs %d", len(withBilling), len(withoutBilling))
}

func TestProbeUsableModelsLive(t *testing.T) {
	if os.Getenv("AQL_LIVE_TEST") != "1" {
		t.Skip("set AQL_LIVE_TEST=1 to run live API tests")
	}
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	usable, err := agent.ProbeUsableModelsWithAPIKey(ctx, apiKey)
	require.NoError(t, err)
	assert.True(t, len(usable) > 0, "should find at least one usable model")

	t.Logf("usable models (%d):", len(usable))
	for _, m := range usable {
		t.Logf("  OK  %s — %s", m.ID, m.DisplayName)
	}
}

func TestProbeUsableModelsLive_OAuthBilling(t *testing.T) {
	if os.Getenv("AQL_LIVE_TEST") != "1" {
		t.Skip("set AQL_LIVE_TEST=1 to run live API tests")
	}

	workDir, _ := os.Getwd()
	tokens, err := loadTokensFromDir(t, workDir)
	if err != nil || tokens == nil {
		t.Skip("no OAuth tokens found — run `aql auth login --console`")
	}
	if tokens.APIKey == "" {
		t.Skip("no API key in tokens — re-run `aql auth login --console`")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	usable, err2 := agent.ProbeUsableModelsWithOAuthKey(ctx, tokens.APIKey)
	require.NoError(t, err2)
	assert.True(t, len(usable) > 0, "should find at least one usable model")

	t.Logf("usable models with OAuth billing (%d):", len(usable))
	var hasOpus, hasSonnet bool
	for _, m := range usable {
		t.Logf("  OK  %s — %s — %dk ctx", m.ID, m.DisplayName, m.MaxInputTokens/1000)
		if contains(m.ID, "opus") {
			hasOpus = true
		}
		if contains(m.ID, "sonnet") {
			hasSonnet = true
		}
	}
	assert.True(t, hasOpus, "OAuth billing should unlock Opus")
	assert.True(t, hasSonnet, "OAuth billing should unlock Sonnet")
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

func TestSaveModelRejectsSlashCommands(t *testing.T) {
	dir := t.TempDir()

	invalidValues := []string{"/exit", "/quit", "/model", "/clear", "/help", ""}
	for _, val := range invalidValues {
		err := agent.SaveModel(dir, val)
		assert.Error(t, err, "should reject %q as model ID", val)
	}
}

func TestSaveModelRejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	err := agent.SaveModel(dir, "")
	assert.Error(t, err, "should reject empty model ID")
}

func TestValidateModelID(t *testing.T) {
	// Valid IDs
	assert.NoError(t, agent.ValidateModelID("claude-sonnet-4-6"))
	assert.NoError(t, agent.ValidateModelID("claude-opus-4-6"))
	assert.NoError(t, agent.ValidateModelID("claude-haiku-4-5-20251001"))
	assert.NoError(t, agent.ValidateModelID("claude-opus-4-6-20260301"))

	// Invalid IDs — slash commands
	assert.Error(t, agent.ValidateModelID("/exit"))
	assert.Error(t, agent.ValidateModelID("/quit"))
	assert.Error(t, agent.ValidateModelID("/model"))

	// Invalid IDs — empty or whitespace
	assert.Error(t, agent.ValidateModelID(""))
	assert.Error(t, agent.ValidateModelID("   "))
}
