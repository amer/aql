package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/amer/aql/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePKCE_VerifierLength(t *testing.T) {
	verifier, _ := auth.GeneratePKCE()
	// base64url of 32 bytes = 43 chars
	assert.Len(t, verifier, 43)
}

func TestGeneratePKCE_ChallengeLength(t *testing.T) {
	_, challenge := auth.GeneratePKCE()
	// SHA256 hash base64url encoded = 43 chars
	assert.Len(t, challenge, 43)
}

func TestGeneratePKCE_Uniqueness(t *testing.T) {
	v1, _ := auth.GeneratePKCE()
	v2, _ := auth.GeneratePKCE()
	assert.NotEqual(t, v1, v2)
}

func TestBuildAuthorizeURL_ConsoleMode(t *testing.T) {
	url := auth.BuildAuthorizeURL("test-challenge", "test-state", 49152, true)

	assert.Contains(t, url, "platform.claude.com/oauth/authorize")
	assert.Contains(t, url, "client_id="+auth.ClientID)
	assert.Contains(t, url, "response_type=code")
	assert.Contains(t, url, "code_challenge=test-challenge")
	assert.Contains(t, url, "code_challenge_method=S256")
	assert.Contains(t, url, "state=test-state")
	assert.Contains(t, url, "code=true")
	assert.Contains(t, url, "redirect_uri=http://localhost:49152/callback")
}

func TestBuildAuthorizeURL_ClaudeAiMode(t *testing.T) {
	url := auth.BuildAuthorizeURL("ch", "st", 8080, false)

	assert.Contains(t, url, "claude.com/cai/oauth/authorize")
	assert.Contains(t, url, "code=true")
}

func TestBuildAuthorizeURL_ScopeNotEncoded(t *testing.T) {
	u := auth.BuildAuthorizeURL("ch", "st", 8080, true)

	// Extract just the scope parameter value
	// The scope must have literal colons, not %3A
	assert.Contains(t, u, "scope=org:create_api_key")
	assert.NotContains(t, u, "scope=org%3A", "colons in scope must not be percent-encoded")
	assert.Contains(t, u, "user:inference")
	assert.Contains(t, u, "org:create_api_key")
	assert.Contains(t, u, "user:profile")
}

func TestBuildAuthorizeURL_AllScopesPresent(t *testing.T) {
	url := auth.BuildAuthorizeURL("ch", "st", 8080, true)

	for _, scope := range auth.AllScopes {
		assert.Contains(t, url, scope, "missing scope: %s", scope)
	}
}

func TestExchangeCode_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "authorization_code", req["grant_type"])
		assert.Equal(t, "test-code", req["code"])
		assert.Equal(t, "test-verifier", req["code_verifier"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access-123",
			"refresh_token": "refresh-456",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	tokens, err := auth.ExchangeCode(server.URL, "test-code", "test-verifier", "st", 8080)
	require.NoError(t, err)
	assert.Equal(t, "access-123", tokens.AccessToken)
	assert.Equal(t, "refresh-456", tokens.RefreshToken)
	assert.False(t, tokens.IsExpired())
}

func TestExchangeCode_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	_, err := auth.ExchangeCode(server.URL, "code", "verifier", "st", 8080)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestRefreshAccessToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "refresh_token", req["grant_type"])
		assert.Equal(t, "old-refresh", req["refresh_token"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    7200,
		})
	}))
	defer server.Close()

	tokens, err := auth.RefreshAccessToken(server.URL, "old-refresh")
	require.NoError(t, err)
	assert.Equal(t, "new-access", tokens.AccessToken)
	assert.Equal(t, "new-refresh", tokens.RefreshToken)
}

func TestTokens_IsExpired(t *testing.T) {
	expired := auth.NewTokens("a", "r", -1)
	assert.True(t, expired.IsExpired())

	valid := auth.NewTokens("a", "r", 3600)
	assert.False(t, valid.IsExpired())
}

func TestTokens_NeedsRefresh(t *testing.T) {
	// Expires in 2 minutes — within the 5-minute refresh threshold
	soon := auth.NewTokens("a", "r", 120)
	assert.True(t, soon.NeedsRefresh())

	// Expires in 1 hour — no refresh needed
	later := auth.NewTokens("a", "r", 3600)
	assert.False(t, later.NeedsRefresh())
}

func TestSaveAndLoadTokens(t *testing.T) {
	dir := t.TempDir()
	original := auth.NewTokens("access-abc", "refresh-xyz", 3600)

	err := auth.SaveTokens(dir, *original)
	require.NoError(t, err)

	loaded, err := auth.LoadTokens(dir)
	require.NoError(t, err)
	assert.Equal(t, "access-abc", loaded.AccessToken)
	assert.Equal(t, "refresh-xyz", loaded.RefreshToken)
}

func TestLoadTokens_NoFile(t *testing.T) {
	dir := t.TempDir()
	tokens, err := auth.LoadTokens(dir)
	assert.NoError(t, err)
	assert.Nil(t, tokens)
}

func TestCreateAPIKey_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-oauth-token", r.Header.Get("Authorization"))

		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "aql", req["name"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"raw_key": "sk-ant-api03-real-key-for-messages",
		})
	}))
	defer server.Close()

	apiKey, err := auth.CreateAPIKey(server.URL, "test-oauth-token")
	require.NoError(t, err)
	assert.Equal(t, "sk-ant-api03-real-key-for-messages", apiKey)
}

func TestCreateAPIKey_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": "insufficient scope"}`))
	}))
	defer server.Close()

	_, err := auth.CreateAPIKey(server.URL, "bad-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestSaveTokens_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	tokens := auth.NewTokens("a", "r", 3600)

	err := auth.SaveTokens(dir, *tokens)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dir, ".aql_tokens.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(),
		"token file must have restrictive permissions")
}
