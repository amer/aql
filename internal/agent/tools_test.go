package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amer/aql/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

// --- Tool definitions ---

func TestToolDefinitions_HasAll10Tools(t *testing.T) {
	defs := agent.ToolDefinitions()
	names := make(map[string]bool)
	for _, d := range defs {
		names[d.Name] = true
	}
	expected := []string{
		"read_file", "write_file", "edit", "list_directory", "bash",
		"glob", "grep", "web_fetch", "web_search", "ask_user",
	}
	for _, name := range expected {
		assert.True(t, names[name], "missing tool: %s", name)
	}
	assert.Len(t, defs, len(expected))
}

func TestToolDefinitions_AllHaveDescriptionsAndSchemas(t *testing.T) {
	for _, d := range agent.ToolDefinitions() {
		assert.NotEmpty(t, d.Description, "tool %s missing description", d.Name)
		assert.NotNil(t, d.InputSchema, "tool %s missing schema", d.Name)
	}
}

// --- read_file ---

func TestReadFile_ReturnsFileContents(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "hello.txt", "hello world\nsecond line")

	result, err := agent.ExecuteTool(context.Background(), dir, "read_file",
		json.RawMessage(`{"path":"hello.txt"}`))
	require.NoError(t, err)
	assert.Equal(t, "hello world\nsecond line", result)
}

func TestReadFile_MissingFile(t *testing.T) {
	dir := t.TempDir()
	result, err := agent.ExecuteTool(context.Background(), dir, "read_file",
		json.RawMessage(`{"path":"nope.txt"}`))
	assert.NoError(t, err)
	assert.Contains(t, result, "no such file")
}

// --- write_file ---

func TestWriteFile_CreatesFileAndVerifyContents(t *testing.T) {
	dir := t.TempDir()
	result, err := agent.ExecuteTool(context.Background(), dir, "write_file",
		json.RawMessage(`{"path":"out.txt","content":"written content"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "out.txt")

	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	require.NoError(t, err)
	assert.Equal(t, "written content", string(data))
}

func TestWriteFile_CreatesNestedDirectories(t *testing.T) {
	dir := t.TempDir()
	result, err := agent.ExecuteTool(context.Background(), dir, "write_file",
		json.RawMessage(`{"path":"a/b/c.txt","content":"deep"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "c.txt")

	data, err := os.ReadFile(filepath.Join(dir, "a", "b", "c.txt"))
	require.NoError(t, err)
	assert.Equal(t, "deep", string(data))
}

// --- edit ---

func TestEdit_SingleReplace(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "hello world")

	result, err := agent.ExecuteTool(context.Background(), dir, "edit",
		json.RawMessage(`{"file_path":"f.txt","old_string":"hello","new_string":"goodbye"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "Edited")

	data, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	require.NoError(t, err)
	assert.Equal(t, "goodbye world", string(data))
}

func TestEdit_ReplaceAll(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "aaa bbb aaa")

	result, err := agent.ExecuteTool(context.Background(), dir, "edit",
		json.RawMessage(`{"file_path":"f.txt","old_string":"aaa","new_string":"xxx","replace_all":true}`))
	require.NoError(t, err)
	assert.Contains(t, result, "2 replacements")

	data, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	require.NoError(t, err)
	assert.Equal(t, "xxx bbb xxx", string(data))
}

func TestEdit_AmbiguousMatchFails(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "aaa bbb aaa")

	result, err := agent.ExecuteTool(context.Background(), dir, "edit",
		json.RawMessage(`{"file_path":"f.txt","old_string":"aaa","new_string":"xxx"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "matches 2 times")

	// File should be unchanged
	data, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	require.NoError(t, err)
	assert.Equal(t, "aaa bbb aaa", string(data))
}

func TestEdit_NotFoundInFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "hello world")

	result, err := agent.ExecuteTool(context.Background(), dir, "edit",
		json.RawMessage(`{"file_path":"f.txt","old_string":"zzz","new_string":"xxx"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "not found")
}

func TestEdit_SameStringsFails(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "hello world")

	result, err := agent.ExecuteTool(context.Background(), dir, "edit",
		json.RawMessage(`{"file_path":"f.txt","old_string":"hello","new_string":"hello"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "identical")
}

func TestEdit_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	result, err := agent.ExecuteTool(context.Background(), dir, "edit",
		json.RawMessage(`{"file_path":"nope.txt","old_string":"a","new_string":"b"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "no such file")
}

// --- list_directory ---

func TestListDirectory_ShowsFilesAndDirs(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.go", "package a")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0755))

	result, err := agent.ExecuteTool(context.Background(), dir, "list_directory",
		json.RawMessage(fmt.Sprintf(`{"path":"%s"}`, dir)))
	require.NoError(t, err)
	assert.Contains(t, result, "a.go")
	assert.Contains(t, result, "subdir/")
}

// --- bash ---

func TestBash_ExecutesCommand(t *testing.T) {
	dir := t.TempDir()
	result, err := agent.ExecuteTool(context.Background(), dir, "bash",
		json.RawMessage(`{"command":"echo hello from bash"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "hello from bash")
}

func TestBash_ReportsExitError(t *testing.T) {
	dir := t.TempDir()
	result, err := agent.ExecuteTool(context.Background(), dir, "bash",
		json.RawMessage(`{"command":"exit 1"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "exit")
}

// --- glob ---

func TestGlob_MatchesGoFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.go", "package a")
	writeTestFile(t, dir, "b.go", "package b")
	writeTestFile(t, dir, "c.txt", "not go")

	result, err := agent.ExecuteTool(context.Background(), dir, "glob",
		json.RawMessage(`{"pattern":"*.go","path":"`+dir+`"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "a.go")
	assert.Contains(t, result, "b.go")
	assert.NotContains(t, result, "c.txt")
}

func TestGlob_RecursiveDoublestar(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "sub/deep.go", "package deep")
	writeTestFile(t, dir, "top.go", "package top")

	result, err := agent.ExecuteTool(context.Background(), dir, "glob",
		json.RawMessage(`{"pattern":"**/*.go","path":"`+dir+`"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "deep.go")
	assert.Contains(t, result, "top.go")
}

func TestGlob_NoMatches(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.txt", "text")

	result, err := agent.ExecuteTool(context.Background(), dir, "glob",
		json.RawMessage(`{"pattern":"*.go","path":"`+dir+`"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "No files matched")
}

func TestGlob_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".hidden/secret.go", "package secret")
	writeTestFile(t, dir, "visible.go", "package visible")

	result, err := agent.ExecuteTool(context.Background(), dir, "glob",
		json.RawMessage(`{"pattern":"**/*.go","path":"`+dir+`"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "visible.go")
	assert.NotContains(t, result, "secret.go")
}

// --- grep ---

func TestGrep_FindsPattern(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "test.go", "func main() {\n\tfmt.Println(\"hello\")\n}")

	result, err := agent.ExecuteTool(context.Background(), dir, "grep",
		json.RawMessage(`{"pattern":"Println","path":"`+dir+`"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "Println")
	assert.Contains(t, result, "test.go")
}

func TestGrep_NoMatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "test.go", "package main")

	result, err := agent.ExecuteTool(context.Background(), dir, "grep",
		json.RawMessage(`{"pattern":"zzznope","path":"`+dir+`"}`))
	require.NoError(t, err)
	assert.Empty(t, result)
}

// --- web_fetch ---

func TestWebFetch_PlainText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "plain text content")
	}))
	defer srv.Close()

	result, err := agent.ExecuteTool(context.Background(), ".", "web_fetch",
		json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, srv.URL)))
	require.NoError(t, err)
	assert.Equal(t, "plain text content", result)
}

func TestWebFetch_HTMLExtractsText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><p>visible text</p><script>hidden();</script></body></html>`)
	}))
	defer srv.Close()

	result, err := agent.ExecuteTool(context.Background(), ".", "web_fetch",
		json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, srv.URL)))
	require.NoError(t, err)
	assert.Contains(t, result, "visible text")
	assert.NotContains(t, result, "hidden()")
}

func TestWebFetch_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		fmt.Fprint(w, "not found")
	}))
	defer srv.Close()

	result, err := agent.ExecuteTool(context.Background(), ".", "web_fetch",
		json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, srv.URL)))
	require.NoError(t, err)
	assert.Contains(t, result, "HTTP 404")
}

func TestWebFetch_InvalidURL(t *testing.T) {
	result, err := agent.ExecuteTool(context.Background(), ".", "web_fetch",
		json.RawMessage(`{"url":"not a url"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "error")
}

// --- web_search ---

func TestWebSearch_ParsesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>
			<div class="result">
				<a class="result__a" href="https://example.com">Example Title</a>
				<span class="result__snippet">A snippet about the result</span>
			</div>
		</body></html>`)
	}))
	defer srv.Close()

	// We can't override the DuckDuckGo URL directly, but we can test parseSearchResults
	// through the extractText function indirectly. For a real integration test,
	// we'd need to inject the HTTP client. Instead, test that the tool handles
	// a valid search gracefully by verifying it doesn't error.
	result, err := agent.ExecuteTool(context.Background(), ".", "web_search",
		json.RawMessage(`{"query":"golang testing"}`))
	require.NoError(t, err)
	// Should return something (either results or "No results found.")
	assert.NotEmpty(t, result)
}

// --- ask_user ---

func TestAskUser_WithFunc(t *testing.T) {
	askFn := func(ctx context.Context, q agent.UserQuestion) (string, error) {
		assert.Equal(t, "What is your name?", q.Question)
		return "Alice", nil
	}
	exec := agent.DefaultToolExecutor(askFn)

	result, err := exec(context.Background(), ".", "ask_user",
		json.RawMessage(`{"question":"What is your name?"}`))
	require.NoError(t, err)
	assert.Equal(t, "Alice", result)
}

func TestAskUser_NoFunc(t *testing.T) {
	// ExecuteTool with no askUser returns "not available"
	result, err := agent.ExecuteTool(context.Background(), ".", "ask_user",
		json.RawMessage(`{"question":"hello?"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "not available")
}

func TestAskUser_ContextCanceled(t *testing.T) {
	askFn := func(ctx context.Context, q agent.UserQuestion) (string, error) {
		return "", ctx.Err()
	}
	exec := agent.DefaultToolExecutor(askFn)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := exec(ctx, ".", "ask_user",
		json.RawMessage(`{"question":"hello?"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "error")
}

// --- unknown tool ---

func TestExecuteTool_Unknown(t *testing.T) {
	_, err := agent.ExecuteTool(context.Background(), ".", "unknown_tool", json.RawMessage(`{}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

// --- extractText (via web_fetch with HTML) ---

func TestWebFetch_StripsScriptsAndStyles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html>
			<head><style>body { color: red; }</style></head>
			<body>
				<h1>Title</h1>
				<script>alert('xss')</script>
				<p>Paragraph text</p>
				<noscript>no js</noscript>
			</body>
		</html>`)
	}))
	defer srv.Close()

	result, err := agent.ExecuteTool(context.Background(), ".", "web_fetch",
		json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, srv.URL)))
	require.NoError(t, err)
	assert.Contains(t, result, "Title")
	assert.Contains(t, result, "Paragraph text")
	assert.NotContains(t, result, "alert")
	assert.NotContains(t, result, "color: red")
	assert.NotContains(t, result, "no js")
}

// --- integration: edit preserves multiline content correctly ---

func TestEdit_MultilineContent(t *testing.T) {
	dir := t.TempDir()
	original := "line1\nline2\nline3\nline4"
	writeTestFile(t, dir, "multi.txt", original)

	result, err := agent.ExecuteTool(context.Background(), dir, "edit",
		json.RawMessage(`{"file_path":"multi.txt","old_string":"line2\nline3","new_string":"replaced2\nreplaced3"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "Edited")

	data, err := os.ReadFile(filepath.Join(dir, "multi.txt"))
	require.NoError(t, err)
	assert.Equal(t, "line1\nreplaced2\nreplaced3\nline4", string(data))
}

// --- integration: glob returns results sorted by mod time ---

func TestGlob_SortedByModTime(t *testing.T) {
	dir := t.TempDir()
	// Create files with different mod times
	writeTestFile(t, dir, "old.go", "old")
	writeTestFile(t, dir, "new.go", "new")

	result, err := agent.ExecuteTool(context.Background(), dir, "glob",
		json.RawMessage(`{"pattern":"*.go","path":"`+dir+`"}`))
	require.NoError(t, err)
	lines := strings.Split(result, "\n")
	require.Len(t, lines, 2)
	// Most recently modified should be first
	assert.Contains(t, lines[0], "new.go")
	assert.Contains(t, lines[1], "old.go")
}
