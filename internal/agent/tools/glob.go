package tools

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - execGlob handler
//   - walkGlob — directory walking
//   - matchesPattern — glob matching entry point (path or base name)
//   - matchGlob, matchSegments — segment-based ** matching
//   - formatGlobResults, fileEntry type, maxGlobResults constant
//
// MUST NOT GO HERE:
//   - Other file operations (file.go)
//   - Shell-based find commands (we do our own walking)
//   - Tool definitions
//
// Q: Why not use filepath.Glob?
// A: It doesn't support ** recursive matching. We walk the tree
//    ourselves.
// ──────────────────────────────────────────────────────────────────

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
	if matchGlob(pattern, relPath) {
		return true
	}
	// A pattern with no directory separator also matches by base name, so
	// "*.ts" finds files at any depth without an explicit "**/" prefix.
	if !strings.Contains(pattern, "/") {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	return false
}

// matchGlob reports whether path matches pattern, treating "**" as a wildcard
// for zero or more path segments. Other segments follow filepath.Match
// semantics, where "*" matches within a single segment only.
func matchGlob(pattern, path string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(path, "/"))
}

// matchSegments matches path segments against pattern segments, expanding "**"
// to span any number of intervening segments.
func matchSegments(pat, path []string) bool {
	if len(pat) == 0 {
		return len(path) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(path); i++ {
			if matchSegments(pat[1:], path[i:]) {
				return true
			}
		}
		return false
	}
	if len(path) == 0 {
		return false
	}
	if matched, _ := filepath.Match(pat[0], path[0]); matched {
		return matchSegments(pat[1:], path[1:])
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
