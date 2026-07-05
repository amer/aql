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
//
// Q: Why does ResolveAPIKeyFromDirs take dirs explicitly?
// A: So tests inject a temp dir and never read the developer's real
//    credentials. defaultTokenSearchDirs() is the production composition.
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const loginTimeout = 2 * time.Minute

// credentialDir returns the directory where OAuth tokens are stored. It uses
// the OS user-config directory so credentials never land in a project working
// directory, where they would risk being committed to git.
func credentialDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config dir: %w", err)
	}
	return filepath.Join(base, "aql"), nil
}

// defaultTokenSearchDirs is the ordered list of directories searched for saved
// tokens in production: the canonical credential dir first, then the home dir
// for backwards compatibility with older versions that saved there. The working
// directory is deliberately excluded — a hostile repo could otherwise commit a
// token file and hijack the session (see credentialDir's own comment).
func defaultTokenSearchDirs() []string {
	var dirs []string
	if dir, err := credentialDir(); err == nil {
		dirs = append(dirs, dir)
	}
	if homeDir, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, homeDir)
	}
	return dirs
}

// loadFirstTokens returns the first readable token file across the search dirs.
// Read failures are logged (not silently swallowed) so a corrupt token file is
// diagnosable rather than looking identical to "no login".
func loadFirstTokens(dirs []string) *Tokens {
	for _, dir := range dirs {
		tokens, err := LoadTokens(dir)
		if err != nil {
			slog.Warn("failed to load tokens", "dir", dir, "err", err)
			continue
		}
		if tokens != nil {
			return tokens
		}
	}
	return nil
}

// ResolveAPIKey determines the API key and authentication method using the
// production token search dirs (see defaultTokenSearchDirs).
func ResolveAPIKey() (apiKey string, isOAuth bool, err error) {
	return ResolveAPIKeyFromDirs(defaultTokenSearchDirs())
}

// ResolveAPIKeyFromDirs checks OAuth tokens across the given dirs (in order),
// falling back to the ANTHROPIC_API_KEY environment variable. Dirs are injected
// so tests stay hermetic and never read the developer's real credentials.
func ResolveAPIKeyFromDirs(dirs []string) (apiKey string, isOAuth bool, err error) {
	tokens := loadFirstTokens(dirs)

	switch {
	case tokens != nil && tokens.IsExpired():
		slog.Warn("OAuth tokens expired, falling back to API key")
	case tokens != nil && tokens.APIKey == "":
		slog.Warn("OAuth tokens present but API key is empty, falling back to API key")
	case tokens != nil:
		slog.Info("using OAuth authentication", "expiresAt", tokens.ExpiresAt)
		return tokens.APIKey, true, nil
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

	dir, err := credentialDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create credential dir: %w", err)
	}
	if err := SaveTokens(dir, *tokens); err != nil {
		return fmt.Errorf("save tokens: %w", err)
	}

	fmt.Printf("Login successful! Tokens saved to %s\n", filepath.Join(dir, tokenFileName))
	fmt.Printf("Token expires at: %s\n", tokens.ExpiresAt.Format("2006-01-02 15:04:05"))
	return nil
}
