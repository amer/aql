package tools

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - File system tool handlers — execReadFile, execWriteFile,
//     execEdit, execListDirectory
//   - All take (workDir, json.RawMessage) and return (string, error)
//
// MUST NOT GO HERE:
//   - Tool definitions (defs.go)
//   - Path resolution logic beyond resolvePath (that's in defs.go)
//   - TUI concerns
//   - Network I/O
//
// Q: Should I add a new file operation tool?
// A: Add the handler here, definition in defs.go, registry entry in
//    buildRegistry().
//
// Q: How do I report errors to the agent?
// A: Return the error message as the string value with nil Go error.
//    Never return a Go error for domain failures.
// ──────────────────────────────────────────────────────────────────

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func execReadFile(workDir string, input json.RawMessage) (string, error) {
	params, errMsg := parseInput[struct {
		Path string `json:"path"`
	}](input)
	if errMsg != "" {
		return errMsg, nil
	}
	data, err := os.ReadFile(resolvePath(workDir, params.Path))
	if err != nil {
		return err.Error(), nil
	}
	return string(data), nil
}

func execWriteFile(workDir string, input json.RawMessage) (string, error) {
	params, errMsg := parseInput[struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}](input)
	if errMsg != "" {
		return errMsg, nil
	}
	path := resolvePath(workDir, params.Path)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err.Error(), nil
	}
	if err := os.WriteFile(path, []byte(params.Content), 0644); err != nil {
		return err.Error(), nil
	}
	return "Wrote " + path, nil
}

func execEdit(workDir string, input json.RawMessage) (string, error) {
	params, errMsg := parseInput[struct {
		FilePath   string `json:"file_path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll bool   `json:"replace_all"`
	}](input)
	if errMsg != "" {
		return errMsg, nil
	}
	if params.OldString == params.NewString {
		return "old_string and new_string are identical — nothing to change", nil
	}
	path := resolvePath(workDir, params.FilePath)
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error(), nil
	}
	content := string(data)
	count := strings.Count(content, params.OldString)
	if count == 0 {
		return "old_string not found in file", nil
	}
	if !params.ReplaceAll && count > 1 {
		return fmt.Sprintf("old_string matches %d times — provide more context to make it unique, or set replace_all to true", count), nil
	}
	var newContent string
	if params.ReplaceAll {
		newContent = strings.ReplaceAll(content, params.OldString, params.NewString)
	} else {
		newContent = strings.Replace(content, params.OldString, params.NewString, 1)
	}
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return err.Error(), nil
	}
	if params.ReplaceAll {
		return fmt.Sprintf("Edited %s (%d replacements)", path, count), nil
	}
	return "Edited " + path, nil
}

func execListDirectory(workDir string, input json.RawMessage) (string, error) {
	params, errMsg := parseInput[struct {
		Path string `json:"path"`
	}](input)
	if errMsg != "" {
		return errMsg, nil
	}
	entries, err := os.ReadDir(resolvePath(workDir, params.Path))
	if err != nil {
		return err.Error(), nil
	}
	var lines []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
	}
	return strings.Join(lines, "\n"), nil
}
