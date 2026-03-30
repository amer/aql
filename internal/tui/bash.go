package tui

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
