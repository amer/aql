package auth

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - ResolveAPIKey() — determines API key source (OAuth tokens → env var)
//   - RunLoginCLI() — handles `aql auth login` subcommand
//
// MUST NOT GO HERE:
//   - OAuth implementation details (oauth.go)
//   - Login flow (login.go)
//   - Agent or TUI imports
//
// Q: Should I add a new auth source?
// A: Add it to ResolveAPIKey() in the fallback chain.
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

const loginTimeout = 2 * time.Minute

// ResolveAPIKey determines the API key and authentication method.
// It checks OAuth tokens in workDir and the user's home directory,
// falling back to the ANTHROPIC_API_KEY environment variable.
func ResolveAPIKey(workDir string) (apiKey string, isOAuth bool, err error) {
	tokens, _ := LoadTokens(workDir)
	if tokens == nil {
		if homeDir, homeErr := os.UserHomeDir(); homeErr == nil {
			tokens, _ = LoadTokens(homeDir)
		}
	}

	if tokens != nil && !tokens.IsExpired() {
		slog.Info("using OAuth authentication", "expiresAt", tokens.ExpiresAt)
		return tokens.APIKey, true, nil
	}
	if tokens != nil && tokens.IsExpired() {
		slog.Warn("OAuth tokens expired, falling back to API key")
	}

	envKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if envKey == "" {
		return "", false, fmt.Errorf("ANTHROPIC_API_KEY is not set\n\n  export ANTHROPIC_API_KEY=<your-key>\n\n  Or run: aql auth login --console")
	}
	return envKey, false, nil
}

// RunLoginCLI runs the `aql auth login` subcommand.
func RunLoginCLI(args []string) error {
	console := false
	for _, arg := range args {
		if arg == "--console" {
			console = true
		}
	}

	fmt.Println("Logging in to Anthropic...")
	if console {
		fmt.Println("Using Console (API billing) login")
	}

	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()

	tokens, err := Login(ctx, LoginOptions{Console: console})
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := SaveTokens(workDir, *tokens); err != nil {
		return fmt.Errorf("save tokens: %w", err)
	}

	fmt.Printf("Login successful! Tokens saved to %s/.aql_tokens.json\n", workDir)
	fmt.Printf("Token expires at: %s\n", tokens.ExpiresAt.Format("2006-01-02 15:04:05"))
	return nil
}
