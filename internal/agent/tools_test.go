package agent_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/amer/aql/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
}

func TestToolDefinitions_HasExpectedTools(t *testing.T) {
	defs := agent.ToolDefinitions()
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	assert.Contains(t, names, "read_file")
	assert.Contains(t, names, "list_directory")
	assert.Contains(t, names, "bash")
	assert.Contains(t, names, "grep")
	assert.Contains(t, names, "write_file")
}

func TestToolDefinitions_HaveDescriptions(t *testing.T) {
	for _, d := range agent.ToolDefinitions() {
		assert.NotEmpty(t, d.Description, "tool %s missing description", d.Name)
	}
}

func TestToolDefinitions_HaveSchemas(t *testing.T) {
	for _, d := range agent.ToolDefinitions() {
		assert.NotNil(t, d.InputSchema, "tool %s missing schema", d.Name)
	}
}

func TestExecuteTool_ReadFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "hello.txt", "hello world")

	input := json.RawMessage(`{"path":"` + dir + `/hello.txt"}`)
	result, err := agent.ExecuteTool(context.Background(), dir, "read_file", input)
	require.NoError(t, err)
	assert.Contains(t, result, "hello world")
}

func TestExecuteTool_ReadFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	input := json.RawMessage(`{"path":"` + dir + `/nope.txt"}`)
	result, err := agent.ExecuteTool(context.Background(), dir, "read_file", input)
	assert.NoError(t, err) // returns error as content, not Go error
	assert.Contains(t, result, "no such file")
}

func TestExecuteTool_ListDirectory(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.go", "package a")
	writeTestFile(t, dir, "b.go", "package b")

	input := json.RawMessage(`{"path":"` + dir + `"}`)
	result, err := agent.ExecuteTool(context.Background(), dir, "list_directory", input)
	require.NoError(t, err)
	assert.Contains(t, result, "a.go")
	assert.Contains(t, result, "b.go")
}

func TestExecuteTool_Bash(t *testing.T) {
	dir := t.TempDir()
	input := json.RawMessage(`{"command":"echo hello from bash"}`)
	result, err := agent.ExecuteTool(context.Background(), dir, "bash", input)
	require.NoError(t, err)
	assert.Contains(t, result, "hello from bash")
}

func TestExecuteTool_Grep(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "test.go", "func main() {\n\tfmt.Println(\"hello\")\n}")

	input := json.RawMessage(`{"pattern":"Println","path":"` + dir + `"}`)
	result, err := agent.ExecuteTool(context.Background(), dir, "grep", input)
	require.NoError(t, err)
	assert.Contains(t, result, "Println")
}

func TestExecuteTool_WriteFile(t *testing.T) {
	dir := t.TempDir()
	input := json.RawMessage(`{"path":"` + dir + `/out.txt","content":"written by tool"}`)
	result, err := agent.ExecuteTool(context.Background(), dir, "write_file", input)
	require.NoError(t, err)
	assert.Contains(t, result, "out.txt")

	// Verify file was actually written
	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	require.NoError(t, err)
	assert.Equal(t, "written by tool", string(data))
}

func TestExecuteTool_Unknown(t *testing.T) {
	dir := t.TempDir()
	_, err := agent.ExecuteTool(context.Background(), dir, "unknown_tool", json.RawMessage(`{}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}
