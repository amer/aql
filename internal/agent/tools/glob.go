package tools

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type fileEntry struct {
	path    string
	modTime time.Time
}

func execGlob(workDir string, input json.RawMessage) (string, error) {
	params, errMsg := parseInput[struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}](input)
	if errMsg != "" {
		return errMsg, nil
	}
	baseDir := workDir
	if params.Path != "" {
		baseDir = resolvePath(workDir, params.Path)
	}

	matches, err := walkGlob(baseDir, params.Pattern)
	if err != nil {
		return err.Error(), nil
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].modTime.After(matches[j].modTime)
	})

	return formatGlobResults(matches), nil
}

// walkGlob walks baseDir and returns files matching pattern.
func walkGlob(baseDir, pattern string) ([]fileEntry, error) {
	var matches []fileEntry
	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != baseDir {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(baseDir, path)
		if !matchesPattern(pattern, relPath, d.Name()) {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		matches = append(matches, fileEntry{path: path, modTime: info.ModTime()})
		return nil
	})
	return matches, err
}

// matchesPattern checks if a file matches a glob pattern by relative path or name.
func matchesPattern(pattern, relPath, name string) bool {
	if matched, _ := filepath.Match(pattern, relPath); matched {
		return true
	}
	if matched, _ := filepath.Match(pattern, name); matched {
		return true
	}
	if strings.Contains(pattern, "**") {
		trimmed := strings.TrimPrefix(pattern, "**/")
		if matched, _ := filepath.Match(trimmed, name); matched {
			return true
		}
		if matched, _ := filepath.Match(trimmed, relPath); matched {
			return true
		}
	}
	return false
}

const maxGlobResults = 500

// formatGlobResults formats matched files as newline-separated paths, truncating at maxGlobResults.
func formatGlobResults(matches []fileEntry) string {
	if len(matches) == 0 {
		return "No files matched."
	}
	var lines []string
	for _, m := range matches {
		if len(lines) >= maxGlobResults {
			lines = append(lines, fmt.Sprintf("... (%d more)", len(matches)-maxGlobResults))
			break
		}
		lines = append(lines, m.path)
	}
	return strings.Join(lines, "\n")
}
