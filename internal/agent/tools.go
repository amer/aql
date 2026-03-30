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

// UserQuestion is sent from the agent to the TUI when ask_user is invoked.
type UserQuestion struct {
	Question string      `json:"question"`
	ToolID   string      `json:"-"`
	Response chan string `json:"-"`
}

// AskUserFunc is called when the agent needs to ask the user a question.
// It blocks until the user responds. Set by the main app to bridge
// between the runner goroutine and the TUI.
var AskUserFunc func(ctx context.Context, q UserQuestion) (string, error)

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
			Name:        "edit",
			Description: "Apply a targeted find/replace edit to a file. More efficient than rewriting the entire file with write_file.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "Absolute or relative path to the file to edit",
					},
					"old_string": map[string]any{
						"type":        "string",
						"description": "The exact text to find and replace. Must match the file content exactly.",
					},
					"new_string": map[string]any{
						"type":        "string",
						"description": "The text to replace old_string with. Must be different from old_string.",
					},
					"replace_all": map[string]any{
						"type":        "boolean",
						"description": "If true, replace all occurrences. If false (default), old_string must be unique in the file.",
					},
				},
				"required": []string{"file_path", "old_string", "new_string"},
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
			Name:        "glob",
			Description: "Find files matching a glob pattern. Returns matching file paths sorted by modification time (newest first).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "Glob pattern to match (e.g. **/*.go, src/**/*.ts, *.json). Supports ** for recursive matching.",
					},
					"path": map[string]any{
						"type":        "string",
						"description": "Base directory to search in. Defaults to working directory.",
					},
				},
				"required": []string{"pattern"},
			},
		},
		{
			Name:        "web_fetch",
			Description: "Fetch the contents of a URL. Returns the page content as text. For HTML pages, extracts readable text content.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{
						"type":        "string",
						"description": "The URL to fetch",
					},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:        "web_search",
			Description: "Search the web for a query. Returns a list of search results with titles, URLs, and snippets.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The search query",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "ask_user",
			Description: "Ask the user a clarifying question when you need more information to proceed. Pauses execution until the user responds.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question": map[string]any{
						"type":        "string",
						"description": "The question to ask the user",
					},
				},
				"required": []string{"question"},
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

// ExecuteToolFunc is the function used to execute tools. Override in tests
// to inject mocks. Default is executeTool.
var ExecuteToolFunc = executeTool

// ExecuteTool runs a tool by name with the given JSON input.
func ExecuteTool(ctx context.Context, workDir string, name string, input json.RawMessage) (string, error) {
	return ExecuteToolFunc(ctx, workDir, name, input)
}

func executeTool(ctx context.Context, workDir string, name string, input json.RawMessage) (string, error) {
	slog.Debug("executing tool", "tool", name, "workDir", workDir)

	switch name {
	case "read_file":
		return execReadFile(workDir, input)
	case "write_file":
		return execWriteFile(workDir, input)
	case "edit":
		return execEdit(workDir, input)
	case "list_directory":
		return execListDirectory(workDir, input)
	case "bash":
		return execBash(ctx, workDir, input)
	case "glob":
		return execGlob(workDir, input)
	case "grep":
		return execGrep(ctx, workDir, input)
	case "web_fetch":
		return execWebFetch(ctx, input)
	case "web_search":
		return execWebSearch(ctx, input)
	case "ask_user":
		return execAskUser(ctx, input)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// parseInput unmarshals JSON tool input into the given struct pointer.
func parseInput[T any](input json.RawMessage) (T, string) {
	var params T
	if err := json.Unmarshal(input, &params); err != nil {
		return params, "invalid input: " + err.Error()
	}
	return params, ""
}

func resolvePath(workDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workDir, path)
}

// --- File tools (read, write, edit, list_directory) ---

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

// --- Shell tools (bash, grep) ---

func execBash(ctx context.Context, workDir string, input json.RawMessage) (string, error) {
	params, errMsg := parseInput[struct {
		Command string `json:"command"`
	}](input)
	if errMsg != "" {
		return errMsg, nil
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
	params, errMsg := parseInput[struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
		Include string `json:"include"`
	}](input)
	if errMsg != "" {
		return errMsg, nil
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
	out, _ := cmd.CombinedOutput()
	result := string(out)
	if len(result) > 10000 {
		result = result[:10000] + "\n... (truncated)"
	}
	return result, nil
}

// --- Ask user tool ---

func execAskUser(ctx context.Context, input json.RawMessage) (string, error) {
	params, errMsg := parseInput[struct {
		Question string `json:"question"`
	}](input)
	if errMsg != "" {
		return errMsg, nil
	}
	if AskUserFunc == nil {
		return "ask_user is not available in this context", nil
	}
	q := UserQuestion{
		Question: params.Question,
		Response: make(chan string, 1),
	}
	answer, err := AskUserFunc(ctx, q)
	if err != nil {
		return "ask_user error: " + err.Error(), nil
	}
	return answer, nil
}
