package e2e

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - HasAPICredentials() — checks for API key or token file,
//     findTokenFile() — locates .aql_tokens.json on disk,
//     tokenFile — the token filename constant.
//
// MUST NOT GO HERE:
//   - Token file copying into workDir (terminal.go — copyTokenFile)
//   - OAuth flow or token refresh (internal/auth/)
//
// Q: Where are credentials checked?
// A: ANTHROPIC_API_KEY env var first, then project root, then home dir.
// ──────────────────────────────────────────────────────────────────

import (
	"os"
	"path/filepath"
)

const tokenFile = ".aql_tokens.json"

// HasAPICredentials reports whether API credentials are available,
// either via ANTHROPIC_API_KEY env var or a .aql_tokens.json file
// in the project root or home directory.
func HasAPICredentials() bool {
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return true
	}
	if _, err := os.Stat(filepath.Join(projectRoot(), tokenFile)); err == nil {
		return true
	}
	home, err := os.UserHomeDir()
	if err == nil {
		if _, err := os.Stat(filepath.Join(home, tokenFile)); err == nil {
			return true
		}
	}
	return false
}

// findTokenFile returns the path to the first .aql_tokens.json found
// (project root, then home), or empty string if none exists.
func findTokenFile() string {
	p := filepath.Join(projectRoot(), tokenFile)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	home, err := os.UserHomeDir()
	if err == nil {
		p = filepath.Join(home, tokenFile)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
