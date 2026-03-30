package agent_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/amer/aql/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunner_BillingHeader verifies that the agent includes the Claude Code
// billing header in the system prompt, adaptive thinking, output_config with
// medium effort, and the required beta headers. These are required for
// accessing Opus/Sonnet via OAuth Console login.
func TestRunner_BillingHeader(t *testing.T) {
	fixture, err := os.ReadFile("testdata/message_hello.json")
	require.NoError(t, err)

	var capturedBody map[string]any
	var capturedBetaHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBetaHeader = r.Header.Get("Anthropic-Beta")

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer server.Close()

	workDir := t.TempDir()
	oauthKey := "sk-ant-api03-test-oauth-key"

	a, err := agent.NewWithOAuthKey(agent.Config{
		Name:         "test",
		Role:         "test",
		SystemPrompt: "Be helpful.",
		Model:        "claude-opus-4-6",
	}, workDir, oauthKey, server.URL)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := a.Run(ctx, "hello")
	for range ch {
	}

	// Verify system prompt includes billing header as first block
	system, ok := capturedBody["system"].([]any)
	require.True(t, ok, "system should be an array of text blocks")
	require.True(t, len(system) >= 2, "system should have at least 2 blocks (billing + actual prompt)")

	firstBlock, ok := system[0].(map[string]any)
	require.True(t, ok, "first system block should be an object")
	assert.Equal(t, "text", firstBlock["type"])
	billingText, _ := firstBlock["text"].(string)
	assert.Contains(t, billingText, "x-anthropic-billing-header:",
		"first system block must contain the billing header")

	// Verify thinking is set to adaptive
	thinking, ok := capturedBody["thinking"].(map[string]any)
	require.True(t, ok, "thinking should be set")
	assert.Equal(t, "adaptive", thinking["type"],
		"thinking must be adaptive for Opus access")

	// Verify output_config has effort: medium
	outputConfig, ok := capturedBody["output_config"].(map[string]any)
	require.True(t, ok, "output_config should be set")
	assert.Equal(t, "medium", outputConfig["effort"],
		"output_config.effort must be medium for Opus access")

	// Verify beta headers
	assert.Contains(t, capturedBetaHeader, "claude-code-20250219",
		"must include claude-code beta header")
	assert.Contains(t, capturedBetaHeader, "interleaved-thinking-2025-05-14",
		"must include interleaved-thinking beta header")
}

// TestRunner_NoBillingHeaderForDirectAPIKey verifies that agents created
// with a direct API key (not OAuth) do NOT include the billing header.
// The billing header is only needed for OAuth Console login to unlock Opus.
func TestRunner_NoBillingHeaderForDirectAPIKey(t *testing.T) {
	fixture, err := os.ReadFile("testdata/stream_hello.sse")
	require.NoError(t, err)

	var capturedBody map[string]any
	var capturedBetaHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBetaHeader = r.Header.Get("Anthropic-Beta")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer server.Close()

	workDir := t.TempDir()

	a, err := agent.NewWithBaseURL(agent.Config{
		Name:         "test",
		Role:         "test",
		SystemPrompt: "Be helpful.",
		Model:        "claude-opus-4-6",
	}, workDir, server.URL)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := a.Run(ctx, "hello")
	for range ch {
	}

	// System should NOT contain billing header for direct API keys
	system, ok := capturedBody["system"].([]any)
	require.True(t, ok, "system should be an array")
	for _, block := range system {
		if b, ok := block.(map[string]any); ok {
			text, _ := b["text"].(string)
			assert.NotContains(t, text, "x-anthropic-billing-header",
				"direct API key agents must NOT include billing header")
		}
	}

	// No Claude Code beta headers for direct API key
	assert.False(t, strings.Contains(capturedBetaHeader, "claude-code-20250219"),
		"direct API key agents must NOT include claude-code beta header")
}
