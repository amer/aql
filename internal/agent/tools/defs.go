package tools

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - ToolDef type, Definitions() — all tool JSON schemas
//   - buildRegistry() — tool name→handler mapping
//   - ExecutorOption pattern (WithAskUser, WithTaskStore, WithAgentSpawner)
//   - NewExecutor/DefaultExecutor/Execute
//   - parseInput generic helper, resolvePath helper
//   - toolHandler type, UserQuestion/AskUserFn/ExecutorFn types
//
// MUST NOT GO HERE:
//   - Individual tool implementation logic (each tool gets its own file)
//   - TUI rendering
//   - Agent construction
//   - Anthropic SDK types
//
// Q: How do I add a new tool?
// A: Three steps: (1) Add definition to Definitions(), (2) Add handler
//    to buildRegistry() or register*Tools(), (3) Add display mapping in
//    tui/transcript.go. The DispatchesAllKnownTools test enforces step 2.
//
// Q: How do tool errors work?
// A: Return error string as first value with nil error. Go error is only
//    for infrastructure failures (unknown tool, context canceled).
//
// Q: Where do I add a dynamic tool (needs runtime state)?
// A: Create a register*Tools() function like registerTaskTools(), call it
//    from NewExecutor().
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
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

// AskUserFn is the signature for a function that asks the user a question.
type AskUserFn func(ctx context.Context, q UserQuestion) (string, error)

// ExecutorFn is the signature for a function that executes a tool by name.
type ExecutorFn func(ctx context.Context, workDir, name string, input json.RawMessage) (string, error)

// Definitions returns the set of tools available to agents.
func Definitions() []ToolDef {
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
		{
			Name:        "agent",
			Description: "Spawn a sub-agent to handle a task independently. The sub-agent has its own conversation context and tool access. Use for parallel research, code exploration, or independent subtasks.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prompt": map[string]any{
						"type":        "string",
						"description": "The task for the sub-agent to perform",
					},
					"description": map[string]any{
						"type":        "string",
						"description": "A short (3-5 word) description of the task",
					},
				},
				"required": []string{"prompt"},
			},
		},
		{
			Name:        "task_create",
			Description: "Create a new task to track a unit of work. Returns the created task with its ID.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"description": map[string]any{
						"type":        "string",
						"description": "A description of the task to create",
					},
				},
				"required": []string{"description"},
			},
		},
		{
			Name:        "task_update",
			Description: "Update the status of an existing task. Valid statuses: pending, in_progress, completed.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
						"type":        "integer",
						"description": "The task ID to update",
					},
					"status": map[string]any{
						"type":        "string",
						"description": "New status: pending, in_progress, or completed",
					},
				},
				"required": []string{"id", "status"},
			},
		},
		{
			Name:        "task_list",
			Description: "List all tracked tasks with their current status.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "notebook_edit",
			Description: "Edit a Jupyter notebook cell. Replaces the source of a cell at the given index, optionally changing the cell type.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path to the .ipynb notebook file",
					},
					"cell_index": map[string]any{
						"type":        "integer",
						"description": "Zero-based index of the cell to edit",
					},
					"new_source": map[string]any{
						"type":        "string",
						"description": "New source content for the cell",
					},
					"cell_type": map[string]any{
						"type":        "string",
						"description": "Optional: change cell type (code or markdown)",
					},
				},
				"required": []string{"path", "cell_index", "new_source"},
			},
		},
	}
}

// toolHandler is the uniform signature for all tool implementations.
type toolHandler func(ctx context.Context, workDir string, input json.RawMessage) (string, error)

// buildRegistry returns a tool name → handler map, binding askFn into ask_user.
func buildRegistry(askFn AskUserFn) map[string]toolHandler {
	// Adapters for tools that don't need ctx or workDir
	withDir := func(fn func(string, json.RawMessage) (string, error)) toolHandler {
		return func(_ context.Context, workDir string, input json.RawMessage) (string, error) {
			return fn(workDir, input)
		}
	}
	withCtx := func(fn func(context.Context, json.RawMessage) (string, error)) toolHandler {
		return func(ctx context.Context, _ string, input json.RawMessage) (string, error) {
			return fn(ctx, input)
		}
	}

	return map[string]toolHandler{
		"read_file":      withDir(execReadFile),
		"write_file":     withDir(execWriteFile),
		"edit":           withDir(execEdit),
		"list_directory": withDir(execListDirectory),
		"glob":           withDir(execGlob),
		"bash":           execBash,
		"grep":           execGrep,
		"web_fetch":      withCtx(execWebFetch),
		"web_search":     withCtx(execWebSearch),
		"ask_user": func(ctx context.Context, _ string, input json.RawMessage) (string, error) {
			return execAskUser(ctx, input, askFn)
		},
		"notebook_edit": withDir(execNotebookEdit),
	}
}

// ExecutorOption configures a tool executor.
type ExecutorOption func(*executorOpts)

type executorOpts struct {
	askFn        AskUserFn
	taskStore    *TaskStore
	agentSpawner AgentSpawner
}

// WithAskUser sets the function called when the agent uses ask_user.
func WithAskUser(fn AskUserFn) ExecutorOption {
	return func(o *executorOpts) { o.askFn = fn }
}

// WithTaskStore sets the task store for task_create/task_update/task_list tools.
func WithTaskStore(s *TaskStore) ExecutorOption {
	return func(o *executorOpts) { o.taskStore = s }
}

// WithAgentSpawner sets the spawner for the agent tool (sub-agent creation).
func WithAgentSpawner(s AgentSpawner) ExecutorOption {
	return func(o *executorOpts) { o.agentSpawner = s }
}

// NewExecutor creates an ExecutorFn with the given options.
func NewExecutor(opts ...ExecutorOption) ExecutorFn {
	var o executorOpts
	for _, opt := range opts {
		opt(&o)
	}
	registry := buildRegistry(o.askFn)
	if o.taskStore != nil {
		registerTaskTools(registry, o.taskStore)
	}
	registerAgentTool(registry, o.agentSpawner)
	return func(ctx context.Context, workDir, name string, input json.RawMessage) (string, error) {
		return execute(ctx, workDir, name, input, registry)
	}
}

// DefaultExecutor returns an ExecutorFn that dispatches to the
// built-in tool implementations, using askFn for the ask_user tool.
func DefaultExecutor(askFn AskUserFn) ExecutorFn {
	return NewExecutor(WithAskUser(askFn), WithTaskStore(NewTaskStore()))
}

// Execute runs a tool by name using the default executor with no ask_user support.
func Execute(ctx context.Context, workDir string, name string, input json.RawMessage) (string, error) {
	exec := NewExecutor(WithTaskStore(NewTaskStore()))
	return exec(ctx, workDir, name, input)
}

func execute(ctx context.Context, workDir, name string, input json.RawMessage, registry map[string]toolHandler) (string, error) {
	slog.Debug("executing tool", "tool", name, "workDir", workDir)
	handler, ok := registry[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return handler(ctx, workDir, input)
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
