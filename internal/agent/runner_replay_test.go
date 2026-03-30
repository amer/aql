package agent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer server.Close()

	workDir := t.TempDir()

	coder, err := agent.NewWithBaseURL(agent.Config{
		Name:         "test-coder",
		Role:         "Go developer",
		SystemPrompt: "Reply with exactly: hello world.",
	}, workDir, server.URL)
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
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			// First call: Claude wants to use a tool
			w.Write([]byte(`{
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
			}`))
		} else {
			// Second call: Claude returns final text after seeing tool result
			w.Write([]byte(`{
				"id": "msg_2",
				"type": "message",
				"role": "assistant",
				"content": [
					{"type": "text", "text": "The command output: tool-works"}
				],
				"model": "claude-sonnet-4-6-20250514",
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 50, "output_tokens": 10}
			}`))
		}
	}))
	defer server.Close()

	workDir := t.TempDir()

	coder, err := agent.NewWithBaseURL(agent.Config{
		Name:         "test",
		Role:         "assistant",
		SystemPrompt: "Use tools.",
	}, workDir, server.URL)
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
