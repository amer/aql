package tui

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - IsBashCommand — detects "!" prefix, ParseBashCommand —
//     extracts shell command.
//
// MUST NOT GO HERE:
//   - Shell execution (that's main.go's onBash callback), rendering,
//     state mutation. These are pure parsing functions.
// ──────────────────────────────────────────────────────────────────

import "strings"

// IsBashCommand returns true if the input starts with "!" (bash mode prefix).
func IsBashCommand(input string) bool {
	return len(input) > 0 && input[0] == '!'
}

// ParseBashCommand extracts the shell command from a "!" prefixed input.
// Returns the command string with leading/trailing whitespace trimmed.
func ParseBashCommand(input string) string {
	if !IsBashCommand(input) {
		return ""
	}
	return strings.TrimSpace(input[1:])
}
