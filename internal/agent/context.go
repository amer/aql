package agent

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - LoadClaudeMD() — reads CLAUDE.md from a single directory,
//     CollectClaudeMD() — deduplicates and concatenates from multiple
//     directories.
//
// MUST NOT GO HERE:
//   - System prompt assembly (that's agent.go's BuildPromptParts)
//   - File writing
//   - Anything beyond reading CLAUDE.md files
//
// Q: Should I add support for reading other config files?
// A: Add a similar Load* function here, then integrate in agent.go's
//    BuildPromptParts.
// ──────────────────────────────────────────────────────────────────

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// LoadClaudeMD reads CLAUDE.md from the given directory.
// Returns empty string if the file does not exist.
func LoadClaudeMD(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// CollectClaudeMD loads CLAUDE.md from multiple directories and
// concatenates them, deduplicating by directory path.
func CollectClaudeMD(dirs ...string) string {
	seen := make(map[string]bool)
	var parts []string

	for _, dir := range dirs {
		abs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true

		content, err := LoadClaudeMD(abs)
		if err != nil {
			// Not-exist is already folded into a nil error, so a real error
			// here (permission, I/O) means we silently dropped project
			// context — surface it rather than swallow it.
			slog.Warn("failed to read CLAUDE.md", "dir", abs, "error", err)
			continue
		}
		if content == "" {
			continue
		}
		parts = append(parts, content)
	}

	return strings.Join(parts, "\n")
}
