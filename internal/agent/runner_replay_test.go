package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/amer/aql/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunnerReplay_MessageFromFixture replays a recorded JSON message response
// without calling the real API.
func TestRunnerReplay_MessageFromFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/message_hello.json")
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveSSE(w, jsonToSSE(fixture))
	}))
	defer server.Close()

	workDir := t.TempDir()

	coder, err := agent.New(agent.Config{
		Name:         "test-coder",
		Role:         "Go developer",
		SystemPrompt: "Reply with exactly: hello world.",
	}, workDir, agent.WithBaseURL(server.URL), agent.WithAPIKey("test-key"))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := coder.Run(ctx, "say hello")

	var texts []string
	var gotDone bool

	for evt := range ch {
		if evt.Error != nil {
			t.Fatalf("unexpected error: %v", evt.Error)
		}
		if evt.Done {
			gotDone = true
			break
		}
		if evt.Text != "" {
			texts = append(texts, evt.Text)
		}
	}

	assert.True(t, gotDone, "should receive Done event")
	require.True(t, len(texts) > 0, "should receive text")
	assert.Equal(t, "hello world", texts[0])
}

// TestRunnerReplay_ToolUse verifies the agent executes tools and continues.
func TestRunnerReplay_ToolUse(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			serveSSE(w, jsonToSSE([]byte(`{
				"id": "msg_1",
				"type": "message",
				"role": "assistant",
				"content": [
					{"type": "text", "text": "Let me check that."},
					{"type": "tool_use", "id": "tu_1", "name": "bash", "input": {"command": "echo tool-works"}}
				],
				"model": "claude-sonnet-4-6-20250514",
				"stop_reason": "tool_use",
				"usage": {"input_tokens": 30, "output_tokens": 20}
			}`)))
		} else {
			serveSSE(w, jsonToSSE([]byte(`{
				"id": "msg_2",
				"type": "message",
				"role": "assistant",
				"content": [
					{"type": "text", "text": "The command output: tool-works"}
				],
				"model": "claude-sonnet-4-6-20250514",
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 50, "output_tokens": 10}
			}`)))
		}
	}))
	defer server.Close()

	workDir := t.TempDir()

	coder, err := agent.New(agent.Config{
		Name:         "test",
		Role:         "assistant",
		SystemPrompt: "Use tools.",
	}, workDir, agent.WithBaseURL(server.URL), agent.WithAPIKey("test-key"))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := coder.Run(ctx, "run echo")

	var texts []string
	var toolCalls []string
	var toolDones []string
	var gotDone bool

	for evt := range ch {
		if evt.Error != nil {
			t.Fatalf("unexpected error: %v", evt.Error)
		}
		if evt.Done {
			gotDone = true
			break
		}
		if evt.Text != "" {
			texts = append(texts, evt.Text)
		}
		if evt.ToolCall != nil {
			toolCalls = append(toolCalls, evt.ToolCall.ToolName)
		}
		if evt.ToolDone != nil {
			toolDones = append(toolDones, evt.ToolDone.Output)
		}
	}

	assert.True(t, gotDone)
	assert.Equal(t, 2, callCount, "should make 2 API calls (tool_use + end_turn)")
	assert.Contains(t, texts, "Let me check that.")
	assert.Contains(t, texts, "The command output: tool-works")
	assert.Equal(t, []string{"bash"}, toolCalls)
	require.Len(t, toolDones, 1)
	assert.Contains(t, toolDones[0], "tool-works")
}

// TestRunnerReplay_ParallelToolExecution verifies that multiple tool_use blocks
// in a single response are all executed and results returned correctly.
func TestRunnerReplay_ParallelToolExecution(t *testing.T) {
	callCount := 0

	// Create test files for the agent to read
	workDir := t.TempDir()
	require.NoError(t, os.WriteFile(workDir+"/a.txt", []byte("content-a"), 0644))
	require.NoError(t, os.WriteFile(workDir+"/b.txt", []byte("content-b"), 0644))
	require.NoError(t, os.WriteFile(workDir+"/c.txt", []byte("content-c"), 0644))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			serveSSE(w, jsonToSSE([]byte(`{
				"id": "msg_1",
				"type": "message",
				"role": "assistant",
				"content": [
					{"type": "text", "text": "Let me read all three files."},
					{"type": "tool_use", "id": "tu_a", "name": "read_file", "input": {"path": "a.txt"}},
					{"type": "tool_use", "id": "tu_b", "name": "read_file", "input": {"path": "b.txt"}},
					{"type": "tool_use", "id": "tu_c", "name": "read_file", "input": {"path": "c.txt"}}
				],
				"model": "claude-sonnet-4-6-20250514",
				"stop_reason": "tool_use",
				"usage": {"input_tokens": 50, "output_tokens": 40}
			}`)))
		} else {
			serveSSE(w, jsonToSSE([]byte(`{
				"id": "msg_2",
				"type": "message",
				"role": "assistant",
				"content": [
					{"type": "text", "text": "I read all three files."}
				],
				"model": "claude-sonnet-4-6-20250514",
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 80, "output_tokens": 10}
			}`)))
		}
	}))
	defer server.Close()

	coder, err := agent.New(agent.Config{
		Name:         "test",
		Role:         "assistant",
		SystemPrompt: "Read files.",
	}, workDir, agent.WithBaseURL(server.URL), agent.WithAPIKey("test-key"))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := coder.Run(ctx, "read all files")

	var toolCalls []string
	toolResults := make(map[string]string) // toolID -> output
	var gotDone bool

	for evt := range ch {
		if evt.Error != nil {
			t.Fatalf("unexpected error: %v", evt.Error)
		}
		if evt.Done {
			gotDone = true
			break
		}
		if evt.ToolCall != nil {
			toolCalls = append(toolCalls, evt.ToolCall.ToolName)
		}
		if evt.ToolDone != nil {
			toolResults[evt.ToolDone.ToolID] = evt.ToolDone.Output
		}
	}

	assert.True(t, gotDone)
	assert.Equal(t, 2, callCount, "should make 2 API calls")

	// All 3 tool calls should have been made
	assert.Len(t, toolCalls, 3)

	// All 3 results should be present with correct content
	assert.Equal(t, "content-a", toolResults["tu_a"])
	assert.Equal(t, "content-b", toolResults["tu_b"])
	assert.Equal(t, "content-c", toolResults["tu_c"])
}

// TestRunnerReplay_ParallelToolExecution_Timing verifies tools run concurrently
// by using slow bash commands. If sequential: ~300ms. If parallel: ~100ms.
func TestRunnerReplay_ParallelToolExecution_Timing(t *testing.T) {
	callCount := 0
	var maxConcurrent atomic.Int32
	var currentConcurrent atomic.Int32

	// Track actual concurrency in tool execution
	mockExecutor := func(ctx context.Context, workDir, name string, input json.RawMessage) (string, error) {
		cur := currentConcurrent.Add(1)
		for {
			old := maxConcurrent.Load()
			if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}
		// Simulate I/O delay
		time.Sleep(50 * time.Millisecond)
		currentConcurrent.Add(-1)
		return "ok", nil
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			serveSSE(w, jsonToSSE([]byte(`{
				"id": "msg_1",
				"type": "message",
				"role": "assistant",
				"content": [
					{"type": "tool_use", "id": "tu_1", "name": "read_file", "input": {"path": "a"}},
					{"type": "tool_use", "id": "tu_2", "name": "read_file", "input": {"path": "b"}},
					{"type": "tool_use", "id": "tu_3", "name": "read_file", "input": {"path": "c"}}
				],
				"model": "claude-sonnet-4-6-20250514",
				"stop_reason": "tool_use",
				"usage": {"input_tokens": 30, "output_tokens": 20}
			}`)))
		} else {
			serveSSE(w, jsonToSSE([]byte(`{
				"id": "msg_2",
				"type": "message",
				"role": "assistant",
				"content": [{"type": "text", "text": "done"}],
				"model": "claude-sonnet-4-6-20250514",
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 50, "output_tokens": 5}
			}`)))
		}
	}))
	defer server.Close()

	coder, err := agent.New(agent.Config{
		Name: "test", Role: "assistant", SystemPrompt: "test",
	}, t.TempDir(), agent.WithBaseURL(server.URL), agent.WithAPIKey("test-key"), agent.WithToolExecutor(mockExecutor))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	ch := coder.Run(ctx, "go")
	for evt := range ch {
		if evt.Error != nil {
			t.Fatalf("unexpected error: %v", evt.Error)
		}
		if evt.Done {
			break
		}
	}
	elapsed := time.Since(start)

	// If parallel: ~50ms (all 3 tools sleep concurrently)
	// If sequential: ~150ms (3 x 50ms)
	assert.Less(t, elapsed, 120*time.Millisecond,
		"3 tools with 50ms each should complete in <120ms if parallel (got %v)", elapsed)
	assert.GreaterOrEqual(t, maxConcurrent.Load(), int32(2),
		"should have at least 2 tools running concurrently")
}
