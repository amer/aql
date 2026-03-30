package agent

import (
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
		if err != nil || content == "" {
			continue
		}
		parts = append(parts, content)
	}

	return strings.Join(parts, "\n")
}
