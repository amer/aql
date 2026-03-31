package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amer/aql/internal/agent"
	"github.com/amer/aql/internal/auth"
	"github.com/amer/aql/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadTokensFromDir tries to load OAuth tokens, walking up to project root.
func loadTokensFromDir(t *testing.T, startDir string) (*auth.Tokens, error) {
	t.Helper()
	dir := startDir
	for {
		tokens, err := auth.LoadTokens(dir)
		if err != nil {
			return nil, err
		}
		if tokens != nil {
			return tokens, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, os.ErrNotExist
}

// TestRunnerLive_StreamsFromAPI calls the real Claude API.
// Only runs when AQL_LIVE_TEST=1 is set — use this to validate the cache
// or re-record fixtures.
func TestRunnerLive_StreamsFromAPI(t *testing.T) {
	if os.Getenv("AQL_LIVE_TEST") != "1" {
		t.Skip("set AQL_LIVE_TEST=1 to run live API tests")
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	workDir := t.TempDir()

	coder, err := agent.New(agent.Config{
		Name:         "test-coder",
		Role:         "Go developer",
		SystemPrompt: "Reply with exactly: hello world. Nothing else.",
	}, workDir)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch := coder.Run(ctx, "say hello")

	var chunks []string
	var gotDone bool
	var gotError error

	for evt := range ch {
		if evt.Error != nil {
			gotError = evt.Error
			break
		}
		if evt.Done {
			gotDone = true
			break
		}
		chunks = append(chunks, evt.Text)
		assert.Equal(t, "test-coder", evt.AgentName)
	}

	require.NoError(t, gotError, "API should not return an error")
	assert.True(t, gotDone, "should receive Done event")
	assert.True(t, len(chunks) > 0, "should receive at least one text chunk")

	var full string
	for _, c := range chunks {
		full += c
	}
	assert.Contains(t, full, "hello")
	t.Logf("received %d chunks, full response: %q", len(chunks), full)
}

// TestRunnerLive_OAuthKeyAccessesOpus verifies that an OAuth-issued API key
// (from `aql auth login --console`) can access Opus. This is the core integration
// test for the OAuth flow — it catches the exact bug where the user logs in
// successfully but the agent still gets 400 because the token isn't used correctly.
//
// Requires: .aql_tokens.json in project root (from `aql auth login --console`).
func TestRunnerLive_OAuthKeyAccessesOpus(t *testing.T) {
	if os.Getenv("AQL_LIVE_TEST") != "1" {
		t.Skip("set AQL_LIVE_TEST=1 to run live API tests")
	}

	// Load OAuth tokens — walk up from cwd to find .aql_tokens.json
	workDir, err := os.Getwd()
	require.NoError(t, err)

	tokens, err := loadTokensFromDir(t, workDir)
	if err != nil {
		t.Skipf("no OAuth tokens found: %v — run `aql auth login --console`", err)
	}

	require.False(t, tokens.IsExpired(),
		"OAuth token is expired — run `aql auth login --console` to refresh")

	if tokens.APIKey == "" {
		t.Skip("no API key in tokens — re-run `aql auth login --console` to get one")
	}

	t.Logf("OAuth token found, expires at %s, API key prefix: %s",
		tokens.ExpiresAt.Format("15:04:05"), tokens.APIKey[:min(15, len(tokens.APIKey))])

	// This is the critical test: use the OAuth-derived API key to access Opus
	a, err := agent.New(agent.Config{
		Name:         "test-oauth-opus",
		Role:         "test",
		SystemPrompt: "Reply with exactly one word: pong",
		Model:        "claude-opus-4-6",
	}, t.TempDir(), agent.WithOAuthKey(tokens.APIKey))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch := a.Run(ctx, "ping")

	var gotDone bool
	var gotError error
	var response string

	for evt := range ch {
		if evt.Error != nil {
			gotError = evt.Error
			break
		}
		if evt.Done {
			gotDone = true
			break
		}
		response += evt.Text
	}

	require.NoError(t, gotError,
		"OAuth-derived API key should access Opus without error")
	assert.True(t, gotDone, "should complete successfully")
	assert.NotEmpty(t, response, "Opus should produce a response")
	t.Logf("Opus responded via OAuth API key: %q", response)
}

// TestRunnerLive_AllModelTiers verifies that each model tier from the
// TUI model picker actually works with the current API key.
// This catches issues where a tier shows in the picker but returns 400/404.
func TestRunnerLive_AllModelTiers(t *testing.T) {
	if os.Getenv("AQL_LIVE_TEST") != "1" {
		t.Skip("set AQL_LIVE_TEST=1 to run live API tests")
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	tiers := []struct {
		name    string
		modelID string
	}{
		{"default-sonnet", "claude-sonnet-4-6"},
		{"opus", "claude-opus-4-6"},
		{"haiku", "claude-haiku-4-5"},
	}

	for _, tier := range tiers {
		t.Run(tier.name, func(t *testing.T) {
			workDir := t.TempDir()

			a, err := agent.New(agent.Config{
				Name:         "test-" + tier.name,
				Role:         "test",
				SystemPrompt: "Reply with exactly one word: pong",
				Model:        tier.modelID,
			}, workDir)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			ch := a.Run(ctx, "ping")

			var gotDone bool
			var gotError error
			var response string

			for evt := range ch {
				if evt.Error != nil {
					gotError = evt.Error
					break
				}
				if evt.Done {
					gotDone = true
					break
				}
				response += evt.Text
			}

			if gotError != nil {
				t.Errorf("model %s returned error: %v\n"+
					"  → Your API key may not have access to this model tier.\n"+
					"  → Check your account at console.anthropic.com",
					tier.modelID, gotError)
				return
			}

			assert.True(t, gotDone, "should complete without error")
			assert.NotEmpty(t, response, "should produce a response")
			t.Logf("model %s responded: %q", tier.modelID, response)
		})
	}
}

// TestRunnerLive_SavedModelWorks verifies that the currently saved model
// in .aql_model actually works with the API. This catches the scenario where
// an invalid value (like "/exit") gets persisted and breaks all future sessions.
func TestRunnerLive_SavedModelWorks(t *testing.T) {
	if os.Getenv("AQL_LIVE_TEST") != "1" {
		t.Skip("set AQL_LIVE_TEST=1 to run live API tests")
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	// Load saved model from project root (if any)
	workDir, err := os.Getwd()
	require.NoError(t, err)

	// Walk up to find project root (where .aql_model lives)
	for workDir != "/" {
		if _, err := os.Stat(workDir + "/.aql_model"); err == nil {
			break
		}
		workDir = workDir[:len(workDir)-len("/"+workDir[len(workDir)-1:])]
	}

	savedModel, err := models.LoadModel(workDir)
	if err != nil || savedModel == "" {
		t.Skip("no .aql_model file found")
	}

	t.Logf("testing saved model: %q", savedModel)

	// Validate it doesn't look like a slash command
	assert.NotContains(t, savedModel, "/",
		"saved model ID must not be a slash command — file is corrupted")

	// Actually try to use it
	a, err := agent.New(agent.Config{
		Name:         "test-saved",
		Role:         "test",
		SystemPrompt: "Reply: ok",
		Model:        savedModel,
	}, t.TempDir())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch := a.Run(ctx, "test")

	var gotError error
	for evt := range ch {
		if evt.Error != nil {
			gotError = evt.Error
			break
		}
		if evt.Done {
			break
		}
	}

	if gotError != nil {
		t.Errorf("saved model %q returned error: %v\n"+
			"  → This model may not be accessible with your API key.\n"+
			"  → Try: echo 'claude-haiku-4-5-20251001' > .aql_model",
			savedModel, gotError)
	}
}
