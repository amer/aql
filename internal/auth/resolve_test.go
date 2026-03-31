package auth_test

import (
	"os"
	"testing"
	"time"

	"github.com/amer/aql/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAPIKey_OAuthFromWorkDir(t *testing.T) {
	dir := t.TempDir()
	tokens := auth.Tokens{
		APIKey:    "oauth-key-123",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	require.NoError(t, auth.SaveTokens(dir, tokens))

	key, isOAuth, err := auth.ResolveAPIKey(dir)
	require.NoError(t, err)
	assert.Equal(t, "oauth-key-123", key)
	assert.True(t, isOAuth)
}

func TestResolveAPIKey_ExpiredOAuthFallsBackToEnv(t *testing.T) {
	dir := t.TempDir()
	tokens := auth.Tokens{
		APIKey:    "expired-key",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, auth.SaveTokens(dir, tokens))

	t.Setenv("ANTHROPIC_API_KEY", "env-key-456")

	key, isOAuth, err := auth.ResolveAPIKey(dir)
	require.NoError(t, err)
	assert.Equal(t, "env-key-456", key)
	assert.False(t, isOAuth)
}

func TestResolveAPIKey_NoTokensUsesEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROPIC_API_KEY", "env-key-789")

	key, isOAuth, err := auth.ResolveAPIKey(dir)
	require.NoError(t, err)
	assert.Equal(t, "env-key-789", key)
	assert.False(t, isOAuth)
}

func TestResolveAPIKey_NoTokensNoEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROPIC_API_KEY", "")

	_, _, err := auth.ResolveAPIKey(dir)
	assert.Error(t, err)
}

func TestResolveAPIKey_OAuthFromHomeDir(t *testing.T) {
	workDir := t.TempDir()
	homeDir := t.TempDir()
	tokens := auth.Tokens{
		APIKey:    "home-oauth-key",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	require.NoError(t, auth.SaveTokens(homeDir, tokens))

	// Override HOME so LoadTokens finds it
	t.Setenv("HOME", homeDir)
	// Clear any env key to ensure OAuth path is taken
	t.Setenv("ANTHROPIC_API_KEY", "")

	key, isOAuth, err := auth.ResolveAPIKey(workDir)
	require.NoError(t, err)
	assert.Equal(t, "home-oauth-key", key)
	assert.True(t, isOAuth)
}

func TestRunLoginCLI_Live(t *testing.T) {
	if os.Getenv("AQL_LIVE_TEST") != "1" {
		t.Skip("set AQL_LIVE_TEST=1 to run live login test")
	}
	// Only run with explicit opt-in since it opens a browser
	err := auth.RunLoginCLI([]string{"--console"})
	assert.NoError(t, err)
}
