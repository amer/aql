package auth

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Tokens struct and lifecycle (NewTokens, IsExpired, NeedsRefresh)
//   - PKCE generation (GeneratePKCE)
//   - OAuth URL building (BuildAuthorizeURL)
//   - Token exchange (ExchangeCode), API key creation (CreateAPIKey)
//   - Token refresh (RefreshAccessToken)
//   - Token persistence (SaveTokens, LoadTokens)
//   - OAuth constants and endpoints
//
// MUST NOT GO HERE:
//   - Login flow orchestration (login.go)
//   - API key resolution logic (resolve.go)
//   - Agent or TUI imports
//
// Q: Should I add a new OAuth scope?
// A: Add it to AllScopes here.
//
// Q: Where do tokens get saved?
// A: SaveTokens writes to .aql_tokens.json with 0600 permissions.
// ──────────────────────────────────────────────────────────────────

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// Claude Code's OAuth client ID (public, not a secret).
	ClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"

	// OAuth endpoints — Claude.ai subscription login.
	AuthorizeURL = "https://claude.com/cai/oauth/authorize"

	// OAuth endpoints — Console login (API billing).
	ConsoleAuthorizeURL = "https://platform.claude.com/oauth/authorize"

	// Token endpoint.
	DefaultTokenURL = "https://platform.claude.com/v1/oauth/token"

	// Token file name.
	tokenFileName = ".aql_tokens.json"

	// Refresh tokens 5 minutes before they expire.
	refreshThreshold = 5 * time.Minute
)

// AllScopes are the OAuth scopes Claude Code requests for full access.
var AllScopes = []string{
	"org:create_api_key",
	"user:profile",
	"user:inference",
	"user:sessions:claude_code",
	"user:mcp_servers",
	"user:file_upload",
}

// Tokens holds the OAuth tokens and the derived API key.
type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	APIKey       string    `json:"api_key,omitempty"` // Created from OAuth token, used for Messages API
}

// NewTokens creates a Tokens with an expiry relative to now.
func NewTokens(accessToken, refreshToken string, expiresInSeconds int) *Tokens {
	return &Tokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(expiresInSeconds) * time.Second),
	}
}

// IsExpired returns true if the access token has expired.
func (t Tokens) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// NeedsRefresh returns true if the token will expire soon and should be refreshed.
func (t Tokens) NeedsRefresh() bool {
	return time.Now().After(t.ExpiresAt.Add(-refreshThreshold))
}

// GeneratePKCE generates a PKCE code verifier and challenge (S256).
func GeneratePKCE() (verifier, challenge string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate random bytes: " + err.Error())
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	hash := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(hash[:])
	return verifier, challenge
}

// BuildAuthorizeURL constructs the OAuth authorization URL with PKCE.
// Set console=true for Console (API billing) login, false for Claude.ai subscription login.
//
// We build the query string manually because Go's url.Values.Encode() percent-encodes
// characters (colons, slashes) that the OAuth server expects as literals.
func BuildAuthorizeURL(challenge, state string, port int, console bool) string {
	baseURL := AuthorizeURL
	if console {
		baseURL = ConsoleAuthorizeURL
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)
	scopeStr := strings.Join(AllScopes, " ")

	return baseURL + "?" + strings.Join([]string{
		"code=true",
		"client_id=" + ClientID,
		"response_type=code",
		"redirect_uri=" + redirectURI,
		"scope=" + scopeStr,
		"code_challenge=" + challenge,
		"code_challenge_method=S256",
		"state=" + state,
	}, "&")
}

// ExchangeCode exchanges an authorization code for tokens.
func ExchangeCode(tokenURL, code, verifier, state string, port int) (*Tokens, error) {
	body := map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"redirect_uri":  fmt.Sprintf("http://localhost:%d/callback", port),
		"client_id":     ClientID,
		"code_verifier": verifier,
		"state":         state,
	}

	jsonBody, _ := json.Marshal(body)
	slog.Debug("exchanging auth code", "tokenURL", tokenURL)
	resp, err := http.Post(tokenURL, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	// Read full body for logging
	respBody, _ := io.ReadAll(resp.Body)
	slog.Debug("token exchange response", "status", resp.StatusCode, "body", string(respBody))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: %d %s: %s", resp.StatusCode, resp.Status, string(respBody))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	slog.Debug("token exchange success",
		"accessTokenPrefix", result.AccessToken[:min(20, len(result.AccessToken))],
		"hasRefresh", result.RefreshToken != "",
		"expiresIn", result.ExpiresIn)

	return NewTokens(result.AccessToken, result.RefreshToken, result.ExpiresIn), nil
}

// CreateAPIKeyURL is the endpoint to create an API key from an OAuth token.
const CreateAPIKeyURL = "https://api.anthropic.com/api/oauth/claude_cli/create_api_key"

// CreateAPIKey uses an OAuth Bearer token to create an API key that can be used
// with the Messages API. The OAuth token must have the org:create_api_key scope.
func CreateAPIKey(createKeyURL, oauthToken string) (string, error) {
	body := map[string]string{
		"name": "aql",
	}
	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", createKeyURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+oauthToken)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create API key request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	slog.Debug("create API key response", "status", resp.StatusCode, "body", string(respBody))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create API key failed: %d %s: %s", resp.StatusCode, resp.Status, string(respBody))
	}

	var result struct {
		RawKey string `json:"raw_key"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("decode API key response: %w", err)
	}

	if result.RawKey == "" {
		return "", fmt.Errorf("empty API key in response: %s", string(respBody))
	}

	return result.RawKey, nil
}

// RefreshAccessToken uses a refresh token to get new tokens.
func RefreshAccessToken(tokenURL, refreshToken string) (*Tokens, error) {
	body := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     ClientID,
	}

	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post(tokenURL, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("token refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed: %d %s", resp.StatusCode, resp.Status)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}

	return NewTokens(result.AccessToken, result.RefreshToken, result.ExpiresIn), nil
}

// SaveTokens persists tokens to disk with restrictive permissions.
func SaveTokens(dir string, tokens Tokens) error {
	data, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("marshal tokens: %w", err)
	}
	path := filepath.Join(dir, tokenFileName)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write tokens: %w", err)
	}
	slog.Debug("saved OAuth tokens", "path", path)
	return nil
}

// LoadTokens reads tokens from disk. Returns nil, nil if the file doesn't exist.
func LoadTokens(dir string) (*Tokens, error) {
	path := filepath.Join(dir, tokenFileName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read tokens: %w", err)
	}

	var tokens Tokens
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("unmarshal tokens: %w", err)
	}
	return &tokens, nil
}
