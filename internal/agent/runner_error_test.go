package agent_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/amer/aql/internal/agent"
	"github.com/amer/aql/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunner_SendsCorrectModelID verifies that the agent sends the configured
// model ID in the API request.
func TestRunner_SendsCorrectModelID(t *testing.T) {
	var capturedModel string

	fixture, err := os.ReadFile("testdata/message_hello.json")
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		if m, ok := req["model"].(string); ok {
			capturedModel = m
		}
		serveSSE(w, jsonToSSE(fixture))
	}))
	defer server.Close()

	tests := []struct {
		name      string
		configMod string
		wantModel string
	}{
		{
			name:      "default resolves to sonnet 4.6",
			configMod: "",
			wantModel: "claude-sonnet-4-6",
		},
		{
			name:      "haiku shortcut",
			configMod: "haiku",
			wantModel: "claude-haiku-4-5",
		},
		{
			name:      "opus shortcut",
			configMod: "opus",
			wantModel: "claude-opus-4-6",
		},
		{
			name:      "sonnet shortcut",
			configMod: "sonnet",
			wantModel: "claude-sonnet-4-6",
		},
		{
			name:      "full model ID passthrough",
			configMod: "claude-opus-4-6-20260301",
			wantModel: "claude-opus-4-6-20260301",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedModel = ""
			workDir := t.TempDir()

			opts := testClientOpts(server.URL)
			a, err := agent.New(agent.Config{
				Name:         "test",
				Role:         "test",
				SystemPrompt: "test",
				Model:        tt.configMod,
			}, workDir, opts...)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ch := a.Run(ctx, "hi")
			for range ch {
			}

			assert.Equal(t, tt.wantModel, capturedModel,
				"model ID sent to API should match resolved config")
		})
	}
}

// TestRunner_InvalidModelID verifies that obviously invalid model IDs
// (slash commands, empty-ish strings) are caught before hitting the API.
func TestRunner_InvalidModelID(t *testing.T) {
	invalidModels := []string{
		"/exit",
		"/quit",
		"/model",
		"/clear",
	}

	for _, model := range invalidModels {
		t.Run(model, func(t *testing.T) {
			resolved := models.ResolveModel(model)
			assert.NotContains(t, resolved, "/",
				"resolved model ID must not contain slash commands")
		})
	}
}

// TestRunner_OAuthTokenSentAsAPIKey verifies that OAuth tokens (sk-ant-oat01-*)
// are sent as x-api-key header, not as Authorization: Bearer.
func TestRunner_OAuthTokenSentAsAPIKey(t *testing.T) {
	fixture, err := os.ReadFile("testdata/message_hello.json")
	require.NoError(t, err)

	var capturedAPIKey string
	var capturedAuthHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAPIKey = r.Header.Get("X-Api-Key")
		capturedAuthHeader = r.Header.Get("Authorization")

		serveSSE(w, jsonToSSE(fixture))
	}))
	defer server.Close()

	workDir := t.TempDir()
	oauthKey := "sk-ant-oat01-test-oauth-key"

	// OAuth keys are passed as Bearer tokens to the adapter, which sends
	// them via the Authorization header. The adapter handles the auth mechanism.
	opts := testOAuthClientOpts(server.URL, oauthKey)
	a, err := agent.New(agent.Config{
		Name:         "test",
		Role:         "test",
		SystemPrompt: "test",
	}, workDir, opts...)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := a.Run(ctx, "hi")
	for range ch {
	}

	// With Bearer token auth, the key should be in the Authorization header
	assert.NotEmpty(t, capturedAuthHeader,
		"OAuth key should be sent via Authorization header")
	assert.Empty(t, capturedAPIKey,
		"OAuth key should NOT be sent as x-api-key when using Bearer auth")
}

// TestRunner_API400Error verifies the agent surfaces a meaningful error
// when the API returns 400 Bad Request.
func TestRunner_API400Error(t *testing.T) {
	fixture, err := os.ReadFile("testdata/error_400.json")
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(fixture)
	}))
	defer server.Close()

	workDir := t.TempDir()

	opts := testClientOpts(server.URL)
	a, err := agent.New(agent.Config{
		Name:         "test",
		Role:         "test",
		SystemPrompt: "test",
		Model:        "claude-opus-4-6",
	}, workDir, opts...)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := a.Run(ctx, "hi")

	var gotError error
	for evt := range ch {
		if evt.Error != nil {
			gotError = evt.Error
			break
		}
	}

	require.Error(t, gotError, "should return an error on 400")
	assert.Contains(t, gotError.Error(), "400")
	assert.Contains(t, gotError.Error(), "aql auth login",
		"400 error should tell user to run aql auth login for full access")
	assert.Contains(t, gotError.Error(), "claude-opus-4-6",
		"400 error should mention the model that failed")
}

// TestRunner_API404ModelNotFound verifies the agent handles model-not-found errors.
func TestRunner_API404ModelNotFound(t *testing.T) {
	fixture, err := os.ReadFile("testdata/error_404_model.json")
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write(fixture)
	}))
	defer server.Close()

	workDir := t.TempDir()

	opts := testClientOpts(server.URL)
	a, err := agent.New(agent.Config{
		Name:         "test",
		Role:         "test",
		SystemPrompt: "test",
		Model:        "/exit",
	}, workDir, opts...)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := a.Run(ctx, "hi")

	var gotError error
	for evt := range ch {
		if evt.Error != nil {
			gotError = evt.Error
			break
		}
	}

	require.Error(t, gotError, "should return an error on 404")
	assert.Contains(t, gotError.Error(), "404")
	assert.Contains(t, gotError.Error(), "not found",
		"404 error should include hint about model not being found")
}
