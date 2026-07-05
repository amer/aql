package auth_test

import (
	"os"
	"testing"
	"time"

	"github.com/amer/aql/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAPIKeyFromDirs_OAuth(t *testing.T) {
	dir := t.TempDir()
	tokens := auth.Tokens{
		APIKey:    "oauth-key-123",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	require.NoError(t, auth.SaveTokens(dir, tokens))

	key, isOAuth, err := auth.ResolveAPIKeyFromDirs([]string{dir})
	require.NoError(t, err)
	assert.Equal(t, "oauth-key-123", key)
	assert.True(t, isOAuth)
}

func TestResolveAPIKeyFromDirs_ExpiredOAuthFallsBackToEnv(t *testing.T) {
	dir := t.TempDir()
	tokens := auth.Tokens{
		APIKey:    "expired-key",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, auth.SaveTokens(dir, tokens))

	t.Setenv("ANTHROPIC_API_KEY", "env-key-456")

	key, isOAuth, err := auth.ResolveAPIKeyFromDirs([]string{dir})
	require.NoError(t, err)
	assert.Equal(t, "env-key-456", key)
	assert.False(t, isOAuth)
}

func TestResolveAPIKeyFromDirs_NoTokensUsesEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROPIC_API_KEY", "env-key-789")

	key, isOAuth, err := auth.ResolveAPIKeyFromDirs([]string{dir})
	require.NoError(t, err)
	assert.Equal(t, "env-key-789", key)
	assert.False(t, isOAuth)
}

func TestResolveAPIKeyFromDirs_NoTokensNoEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROPIC_API_KEY", "")

	_, _, err := auth.ResolveAPIKeyFromDirs([]string{dir})
	assert.Error(t, err)
}

func TestResolveAPIKeyFromDirs_EmptyOAuthKeyFallsBackToEnv(t *testing.T) {
	dir := t.TempDir()
	// Unexpired tokens but with an empty API key (e.g. a partially written
	// file, or a subscription login where key creation never ran).
	tokens := auth.Tokens{
		APIKey:    "",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	require.NoError(t, auth.SaveTokens(dir, tokens))

	t.Setenv("ANTHROPIC_API_KEY", "env-fallback-key")

	key, isOAuth, err := auth.ResolveAPIKeyFromDirs([]string{dir})
	require.NoError(t, err)
	assert.Equal(t, "env-fallback-key", key)
	assert.False(t, isOAuth)
}

func TestResolveAPIKeyFromDirs_SearchesInOrder(t *testing.T) {
	// The first dir holding valid tokens wins; later dirs are not consulted.
	first := t.TempDir()
	second := t.TempDir()
	require.NoError(t, auth.SaveTokens(second, auth.Tokens{
		APIKey:    "second-dir-key",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}))
	t.Setenv("ANTHROPIC_API_KEY", "")

	key, isOAuth, err := auth.ResolveAPIKeyFromDirs([]string{first, second})
	require.NoError(t, err)
	assert.Equal(t, "second-dir-key", key)
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
