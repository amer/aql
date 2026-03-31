package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultExecutor_DispatchesAllKnownTools(t *testing.T) {
	// Verify that every tool in Definitions() is reachable via DefaultExecutor.
	// We don't care about the output — just that it doesn't return "unknown tool".
	dir := t.TempDir()
	writeTestFile(t, dir, "test.go", "package main")

	exec := tools.DefaultExecutor(nil)

	for _, def := range tools.Definitions() {
		t.Run(def.Name, func(t *testing.T) {
			// Build minimal valid input for each tool
			input := minimalInput(def.Name, dir)
			_, err := exec(context.Background(), dir, def.Name, input)
			// ask_user returns a soft error string, not a hard error
			if def.Name == "ask_user" {
				assert.NoError(t, err)
				return
			}
			// No "unknown tool" error
			assert.NoError(t, err)
		})
	}
}

func TestDefaultExecutor_UnknownToolReturnsError(t *testing.T) {
	exec := tools.DefaultExecutor(nil)
	_, err := exec(context.Background(), ".", "nonexistent_tool", json.RawMessage(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestDefaultExecutor_AskUserWithFn(t *testing.T) {
	askFn := func(ctx context.Context, q tools.UserQuestion) (string, error) {
		return "answer: " + q.Question, nil
	}
	exec := tools.DefaultExecutor(askFn)
	result, err := exec(context.Background(), ".", "ask_user",
		json.RawMessage(`{"question":"color?"}`))
	require.NoError(t, err)
	assert.Equal(t, "answer: color?", result)
}

// minimalInput returns the smallest valid JSON input for each tool.
func minimalInput(toolName, dir string) json.RawMessage {
	switch toolName {
	case "read_file":
		return json.RawMessage(`{"path":"test.go"}`)
	case "write_file":
		return json.RawMessage(`{"path":"out.txt","content":"x"}`)
	case "edit":
		return json.RawMessage(`{"file_path":"test.go","old_string":"package","new_string":"package"}`)
	case "list_directory":
		return json.RawMessage(`{"path":"` + dir + `"}`)
	case "bash":
		return json.RawMessage(`{"command":"true"}`)
	case "glob":
		return json.RawMessage(`{"pattern":"*.go"}`)
	case "grep":
		return json.RawMessage(`{"pattern":"main"}`)
	case "web_fetch":
		return json.RawMessage(`{"url":"http://invalid.test"}`)
	case "web_search":
		return json.RawMessage(`{"query":"test"}`)
	case "ask_user":
		return json.RawMessage(`{"question":"hello?"}`)
	case "task_create":
		return json.RawMessage(`{"description":"test task"}`)
	case "task_update":
		return json.RawMessage(`{"id":1,"status":"pending"}`)
	case "task_list":
		return json.RawMessage(`{}`)
	case "agent":
		return json.RawMessage(`{"prompt":"hello","description":"test"}`)
	case "notebook_edit":
		return json.RawMessage(`{"path":"missing.ipynb","cell_index":0,"new_source":"x"}`)
	default:
		return json.RawMessage(`{}`)
	}
}
