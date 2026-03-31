package agent

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

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
	type fileEntry struct {
		path    string
		modTime time.Time
	}
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
		matched, _ := filepath.Match(params.Pattern, relPath)
		if !matched {
			matched, _ = filepath.Match(params.Pattern, d.Name())
		}
		if !matched && strings.Contains(params.Pattern, "**") {
			trimmed := strings.TrimPrefix(params.Pattern, "**/")
			matched, _ = filepath.Match(trimmed, d.Name())
			if !matched {
				matched, _ = filepath.Match(trimmed, relPath)
			}
		}
		if matched {
			info, infoErr := d.Info()
			if infoErr != nil {
				return nil
			}
			matches = append(matches, fileEntry{path: path, modTime: info.ModTime()})
		}
		return nil
	})
	if err != nil {
		return err.Error(), nil
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].modTime.After(matches[j].modTime)
	})
	if len(matches) == 0 {
		return "No files matched.", nil
	}
	var lines []string
	for _, m := range matches {
		if len(lines) >= 500 {
			lines = append(lines, fmt.Sprintf("... (%d more)", len(matches)-500))
			break
		}
		lines = append(lines, m.path)
	}
	return strings.Join(lines, "\n"), nil
}
