package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// execTool runs a single tool with a default executor (no ask_user, fresh task
// store). It exists so tests can invoke a tool by name without production code
// exposing a public one-shot Execute that only tests need.
func execTool(ctx context.Context, workDir, name string, input json.RawMessage) (string, error) {
	// Tests fetch httptest servers on 127.0.0.1, so the default SSRF guard
	// (which blocks loopback) is replaced with a permissive one here.
	return tools.NewExecutor(
		tools.WithTaskStore(tools.NewTaskStore()),
		tools.WithFetchGuard(func(*url.URL) string { return "" }),
	)(ctx, workDir, name, input)
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

// --- Tool definitions ---

func TestDefinitions_HasAllTools(t *testing.T) {
	defs := tools.Definitions()
	names := make(map[string]bool)
	for _, d := range defs {
		names[d.Name] = true
	}
	expected := []string{
		"read_file", "write_file", "edit", "list_directory", "bash",
		"glob", "grep", "web_fetch", "web_search", "ask_user",
		"agent", "task_create", "task_update", "task_list",
		"notebook_edit",
	}
	for _, name := range expected {
		assert.True(t, names[name], "missing tool: %s", name)
	}
	assert.Len(t, defs, len(expected))
}

func TestDefinitions_AllHaveDescriptionsAndSchemas(t *testing.T) {
	for _, d := range tools.Definitions() {
		assert.NotEmpty(t, d.Description, "tool %s missing description", d.Name)
		assert.NotNil(t, d.InputSchema, "tool %s missing schema", d.Name)
	}
}

// TestDefinitions_MatchesGolden pins the exact tool definitions (names,
// descriptions, JSON schemas) the LLM sees. It guards against accidental
// wording or schema drift and lets Definitions() be refactored safely: the
// serialized output must stay byte-for-byte identical.
func TestDefinitions_MatchesGolden(t *testing.T) {
	got, err := json.MarshalIndent(tools.Definitions(), "", "  ")
	require.NoError(t, err)
	got = append(got, '\n')

	want, err := os.ReadFile("testdata/definitions.golden.json")
	require.NoError(t, err)

	assert.Equal(t, string(want), string(got),
		"tool definitions changed; if intentional, regenerate testdata/definitions.golden.json")
}

// ToAPITools moved to internal/llm adapter — tested via llm package

// --- read_file ---

func TestReadFile_ReturnsFileContents(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "hello.txt", "hello world\nsecond line")

	result, err := execTool(context.Background(), dir, "read_file",
		json.RawMessage(`{"path":"hello.txt"}`))
	require.NoError(t, err)
	assert.Equal(t, "hello world\nsecond line", result)
}

func TestReadFile_MissingFile(t *testing.T) {
	dir := t.TempDir()
	result, err := execTool(context.Background(), dir, "read_file",
		json.RawMessage(`{"path":"nope.txt"}`))
	assert.NoError(t, err)
	assert.Contains(t, result, "no such file")
}

// --- write_file ---

func TestWriteFile_CreatesFileAndVerifyContents(t *testing.T) {
	dir := t.TempDir()
	result, err := execTool(context.Background(), dir, "write_file",
		json.RawMessage(`{"path":"out.txt","content":"written content"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "out.txt")

	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	require.NoError(t, err)
	assert.Equal(t, "written content", string(data))
}

func TestWriteFile_CreatesNestedDirectories(t *testing.T) {
	dir := t.TempDir()
	result, err := execTool(context.Background(), dir, "write_file",
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

	result, err := execTool(context.Background(), dir, "edit",
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

	result, err := execTool(context.Background(), dir, "edit",
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

	result, err := execTool(context.Background(), dir, "edit",
		json.RawMessage(`{"file_path":"f.txt","old_string":"aaa","new_string":"xxx"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "matches 2 times")

	data, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	require.NoError(t, err)
	assert.Equal(t, "aaa bbb aaa", string(data))
}

func TestEdit_NotFoundInFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "hello world")

	result, err := execTool(context.Background(), dir, "edit",
		json.RawMessage(`{"file_path":"f.txt","old_string":"zzz","new_string":"xxx"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "not found")
}

func TestEdit_SameStringsFails(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "hello world")

	result, err := execTool(context.Background(), dir, "edit",
		json.RawMessage(`{"file_path":"f.txt","old_string":"hello","new_string":"hello"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "identical")
}

func TestEdit_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	result, err := execTool(context.Background(), dir, "edit",
		json.RawMessage(`{"file_path":"nope.txt","old_string":"a","new_string":"b"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "no such file")
}

func TestEdit_MultilineContent(t *testing.T) {
	dir := t.TempDir()
	original := "line1\nline2\nline3\nline4"
	writeTestFile(t, dir, "multi.txt", original)

	result, err := execTool(context.Background(), dir, "edit",
		json.RawMessage(`{"file_path":"multi.txt","old_string":"line2\nline3","new_string":"replaced2\nreplaced3"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "Edited")

	data, err := os.ReadFile(filepath.Join(dir, "multi.txt"))
	require.NoError(t, err)
	assert.Equal(t, "line1\nreplaced2\nreplaced3\nline4", string(data))
}

// --- list_directory ---

func TestListDirectory_ShowsFilesAndDirs(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.go", "package a")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0755))

	result, err := execTool(context.Background(), dir, "list_directory",
		json.RawMessage(fmt.Sprintf(`{"path":"%s"}`, dir)))
	require.NoError(t, err)
	assert.Contains(t, result, "a.go")
	assert.Contains(t, result, "subdir/")
}

// --- bash ---

func TestBash_ExecutesCommand(t *testing.T) {
	dir := t.TempDir()
	result, err := execTool(context.Background(), dir, "bash",
		json.RawMessage(`{"command":"echo hello from bash"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "hello from bash")
}

func TestBash_ReportsExitError(t *testing.T) {
	dir := t.TempDir()
	result, err := execTool(context.Background(), dir, "bash",
		json.RawMessage(`{"command":"exit 1"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "exit")
}

func TestBash_CancelReturnsDespiteOrphanHoldingPipe(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())

	// The command keeps sh alive on a foreground sleep while a backgrounded
	// grandchild inherits and holds open the stdout pipe. Killing only sh on
	// cancel leaves the grandchild holding the pipe, so CombinedOutput blocks
	// until the 60s sleep exits. The tool must bound the wait and return.
	input := json.RawMessage(`{"command":"sleep 60 & echo started; sleep 60"}`)

	done := make(chan struct{})
	go func() {
		execTool(ctx, dir, "bash", input)
		close(done)
	}()

	time.Sleep(200 * time.Millisecond) // let the shell background the grandchild
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("execBash blocked after context cancel — orphan grandchild held the pipe")
	}
}

// --- glob ---

func TestGlob_MatchesGoFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.go", "package a")
	writeTestFile(t, dir, "b.go", "package b")
	writeTestFile(t, dir, "c.txt", "not go")

	result, err := execTool(context.Background(), dir, "glob",
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

	result, err := execTool(context.Background(), dir, "glob",
		json.RawMessage(`{"pattern":"**/*.go","path":"`+dir+`"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "deep.go")
	assert.Contains(t, result, "top.go")
}

func TestGlob_NoMatches(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.txt", "text")

	result, err := execTool(context.Background(), dir, "glob",
		json.RawMessage(`{"pattern":"*.go","path":"`+dir+`"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "No files matched")
}

func TestGlob_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".hidden/secret.go", "package secret")
	writeTestFile(t, dir, "visible.go", "package visible")

	result, err := execTool(context.Background(), dir, "glob",
		json.RawMessage(`{"pattern":"**/*.go","path":"`+dir+`"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "visible.go")
	assert.NotContains(t, result, "secret.go")
}

func TestGlob_SortedByModTime(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "old.go", "old")
	writeTestFile(t, dir, "new.go", "new")

	result, err := execTool(context.Background(), dir, "glob",
		json.RawMessage(`{"pattern":"*.go","path":"`+dir+`"}`))
	require.NoError(t, err)
	lines := strings.Split(result, "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], "new.go")
	assert.Contains(t, lines[1], "old.go")
}

// --- grep ---

func TestGrep_FindsPattern(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "test.go", "func main() {\n\tfmt.Println(\"hello\")\n}")

	result, err := execTool(context.Background(), dir, "grep",
		json.RawMessage(`{"pattern":"Println","path":"`+dir+`"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "Println")
	assert.Contains(t, result, "test.go")
}

func TestGrep_NoMatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "test.go", "package main")

	result, err := execTool(context.Background(), dir, "grep",
		json.RawMessage(`{"pattern":"zzznope","path":"`+dir+`"}`))
	require.NoError(t, err)
	assert.Empty(t, result)
}

// --- WithHTTPClient ---

func TestWithHTTPClient_UsedByWebFetch(t *testing.T) {
	var transportUsed bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	customClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			transportUsed = true
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	exec := tools.NewExecutor(
		tools.WithHTTPClient(customClient),
		tools.WithFetchGuard(func(*url.URL) string { return "" }),
	)
	result, err := exec(context.Background(), ".", "web_fetch",
		json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, srv.URL)))

	require.NoError(t, err)
	assert.Equal(t, "ok", result)
	assert.True(t, transportUsed, "custom HTTP transport should have been used")
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// --- web_fetch ---

func TestWebFetch_BlocksLinkLocalMetadataAddress(t *testing.T) {
	// A default executor keeps the SSRF guard enabled. 169.254.169.254 is the
	// cloud metadata endpoint (a link-local address) the agent must never reach.
	exec := tools.NewExecutor(tools.WithTaskStore(tools.NewTaskStore()))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := exec(ctx, ".", "web_fetch",
		json.RawMessage(`{"url":"http://169.254.169.254/latest/meta-data/"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "refusing")
}

func TestWebFetch_PlainText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "plain text content")
	}))
	defer srv.Close()

	result, err := execTool(context.Background(), ".", "web_fetch",
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

	result, err := execTool(context.Background(), ".", "web_fetch",
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

	result, err := execTool(context.Background(), ".", "web_fetch",
		json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, srv.URL)))
	require.NoError(t, err)
	assert.Contains(t, result, "HTTP 404")
}

func TestWebFetch_InvalidURL(t *testing.T) {
	result, err := execTool(context.Background(), ".", "web_fetch",
		json.RawMessage(`{"url":"not a url"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "error")
}

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

	result, err := execTool(context.Background(), ".", "web_fetch",
		json.RawMessage(fmt.Sprintf(`{"url":"%s"}`, srv.URL)))
	require.NoError(t, err)
	assert.Contains(t, result, "Title")
	assert.Contains(t, result, "Paragraph text")
	assert.NotContains(t, result, "alert")
	assert.NotContains(t, result, "color: red")
	assert.NotContains(t, result, "no js")
}

// --- web_search ---

func TestWebSearch_ParsesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>
			<a class="result__a" href="https://example.com/go-testing">Go Testing Guide</a>
			<a class="result__snippet">Learn how to test in Go</a>
		</body></html>`)
	}))
	defer srv.Close()

	// Use a custom client that rewrites the DuckDuckGo URL to our test server
	customClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	exec := tools.NewExecutor(tools.WithHTTPClient(customClient))
	result, err := exec(context.Background(), ".", "web_search",
		json.RawMessage(`{"query":"golang testing"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "Go Testing Guide")
}

// --- ask_user ---

func TestAskUser_WithFunc(t *testing.T) {
	askFn := func(ctx context.Context, q tools.UserQuestion) (string, error) {
		assert.Equal(t, "What is your name?", q.Question)
		return "Alice", nil
	}
	exec := newDefaultExecutor(askFn)

	result, err := exec(context.Background(), ".", "ask_user",
		json.RawMessage(`{"question":"What is your name?"}`))
	require.NoError(t, err)
	assert.Equal(t, "Alice", result)
}

func TestAskUser_NoFunc(t *testing.T) {
	result, err := execTool(context.Background(), ".", "ask_user",
		json.RawMessage(`{"question":"hello?"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "not available")
}

func TestAskUser_ContextCanceled(t *testing.T) {
	askFn := func(ctx context.Context, q tools.UserQuestion) (string, error) {
		return "", ctx.Err()
	}
	exec := newDefaultExecutor(askFn)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := exec(ctx, ".", "ask_user",
		json.RawMessage(`{"question":"hello?"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "error")
}

// --- unknown tool ---

func TestExecute_Unknown(t *testing.T) {
	_, err := execTool(context.Background(), ".", "unknown_tool", json.RawMessage(`{}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}
