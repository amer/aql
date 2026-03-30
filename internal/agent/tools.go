package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// ToolDef is the pure definition of a tool — name, description, and JSON Schema.
type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// ToolDefinitions returns the set of tools available to agents.
func ToolDefinitions() []ToolDef {
	return []ToolDef{
		{
			Name:        "read_file",
			Description: "Read the contents of a file at the given path. Returns the file content as text.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Absolute or relative path to the file to read",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file at the given path. Creates the file if it doesn't exist, overwrites if it does.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Absolute or relative path to the file to write",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "Content to write to the file",
					},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "list_directory",
			Description: "List the files and directories at the given path. Returns one entry per line.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Absolute or relative path to the directory to list",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "bash",
			Description: "Execute a shell command and return its combined stdout/stderr output. Use this for running tests, builds, git commands, etc.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "The shell command to execute",
					},
				},
				"required": []string{"command"},
			},
		},
		{
			Name:        "grep",
			Description: "Search for a pattern in files. Returns matching lines with file paths and line numbers.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "Regular expression pattern to search for",
					},
					"path": map[string]any{
						"type":        "string",
						"description": "Directory or file to search in",
					},
					"include": map[string]any{
						"type":        "string",
						"description": "Glob pattern to filter files (e.g. *.go)",
					},
				},
				"required": []string{"pattern"},
			},
		},
	}
}

// ToAPITools converts tool definitions to the Anthropic API format.
func ToAPITools(defs []ToolDef) []anthropic.ToolUnionParam {
	tools := make([]anthropic.ToolUnionParam, len(defs))
	for i, d := range defs {
		tools[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        d.Name,
				Description: anthropic.String(d.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: d.InputSchema["properties"],
					Required:   toStringSlice(d.InputSchema["required"]),
				},
			},
		}
	}
	return tools
}

func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		result := make([]string, len(s))
		for i, item := range s {
			result[i] = fmt.Sprint(item)
		}
		return result
	}
	return nil
}

// ExecuteTool runs a tool by name with the given JSON input.
// Tool errors (file not found, command failure) are returned as content strings,
// not Go errors — only truly unknown tools return a Go error.
func ExecuteTool(ctx context.Context, workDir string, name string, input json.RawMessage) (string, error) {
	slog.Debug("executing tool", "tool", name, "workDir", workDir)

	switch name {
	case "read_file":
		return execReadFile(workDir, input)
	case "write_file":
		return execWriteFile(workDir, input)
	case "list_directory":
		return execListDirectory(workDir, input)
	case "bash":
		return execBash(ctx, workDir, input)
	case "grep":
		return execGrep(ctx, workDir, input)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func resolvePath(workDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workDir, path)
}

func execReadFile(workDir string, input json.RawMessage) (string, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "invalid input: " + err.Error(), nil
	}
	data, err := os.ReadFile(resolvePath(workDir, params.Path))
	if err != nil {
		return err.Error(), nil
	}
	return string(data), nil
}

func execWriteFile(workDir string, input json.RawMessage) (string, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "invalid input: " + err.Error(), nil
	}
	path := resolvePath(workDir, params.Path)
	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err.Error(), nil
	}
	if err := os.WriteFile(path, []byte(params.Content), 0644); err != nil {
		return err.Error(), nil
	}
	return "Wrote " + path, nil
}

func execListDirectory(workDir string, input json.RawMessage) (string, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "invalid input: " + err.Error(), nil
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

func execBash(ctx context.Context, workDir string, input json.RawMessage) (string, error) {
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "invalid input: " + err.Error(), nil
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", params.Command)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	result := string(out)
	if err != nil {
		result += "\nexit: " + err.Error()
	}
	return result, nil
}

func execGrep(ctx context.Context, workDir string, input json.RawMessage) (string, error) {
	var params struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
		Include string `json:"include"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "invalid input: " + err.Error(), nil
	}
	searchPath := workDir
	if params.Path != "" {
		searchPath = resolvePath(workDir, params.Path)
	}
	args := []string{"-rn", params.Pattern, searchPath}
	if params.Include != "" {
		args = []string{"-rn", "--include=" + params.Include, params.Pattern, searchPath}
	}
	cmd := exec.CommandContext(ctx, "grep", args...)
	cmd.Dir = workDir
	out, _ := cmd.CombinedOutput() // grep exits 1 on no match — that's fine
	result := string(out)
	// Truncate very long results
	if len(result) > 10000 {
		result = result[:10000] + "\n... (truncated)"
	}
	return result, nil
}
