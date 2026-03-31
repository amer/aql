package agent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amer/aql/internal/agent"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatHistoryForCompaction_EmptyHistory(t *testing.T) {
	result := agent.FormatHistoryForCompaction(nil)
	assert.Equal(t, "", result)
}

func TestFormatHistoryForCompaction_BasicConversation(t *testing.T) {
	history := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock("Hi there!")),
		anthropic.NewUserMessage(anthropic.NewTextBlock("Write tests")),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock("Done.")),
	}

	result := agent.FormatHistoryForCompaction(history)
	assert.Contains(t, result, "User: Hello")
	assert.Contains(t, result, "Assistant: Hi there!")
	assert.Contains(t, result, "User: Write tests")
	assert.Contains(t, result, "Assistant: Done.")
}

func TestFormatHistoryForCompaction_WithToolUse(t *testing.T) {
	history := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Read main.go")),
		anthropic.NewAssistantMessage(
			anthropic.NewToolUseBlock("tool_1", map[string]any{"path": "main.go"}, "read_file"),
		),
		anthropic.NewUserMessage(
			anthropic.NewToolResultBlock("tool_1", "package main\nfunc main() {}", false),
		),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock("Here is main.go")),
	}

	result := agent.FormatHistoryForCompaction(history)
	assert.Contains(t, result, "User: Read main.go")
	assert.Contains(t, result, "[Tool: read_file]")
	assert.Contains(t, result, "[Tool Result]")
	assert.Contains(t, result, "Assistant: Here is main.go")
}

func TestFormatHistoryForCompaction_MultipleBlocksPerMessage(t *testing.T) {
	history := []anthropic.MessageParam{
		anthropic.NewAssistantMessage(
			anthropic.NewTextBlock("I'll read the file."),
			anthropic.NewToolUseBlock("tool_1", map[string]any{"path": "go.mod"}, "read_file"),
		),
	}

	result := agent.FormatHistoryForCompaction(history)
	assert.Contains(t, result, "I'll read the file.")
	assert.Contains(t, result, "[Tool: read_file]")
}

// CompactHistory

const compactSummaryJSON = `{
  "id": "msg_compact",
  "type": "message",
  "role": "assistant",
  "content": [{"type": "text", "text": "The user asked to write auth tests. The assistant created test files."}],
  "model": "claude-sonnet-4-6-20250514",
  "stop_reason": "end_turn",
  "usage": {"input_tokens": 100, "output_tokens": 20}
}`

func newCompactTestAgent(t *testing.T, serverURL string) *agent.Agent {
	t.Helper()
	a, err := agent.NewWithBaseURL(agent.Config{
		Name:         "test-coder",
		Role:         "Go developer",
		SystemPrompt: "You are a Go developer.",
	}, t.TempDir(), serverURL)
	require.NoError(t, err)
	return a
}

func TestCompactHistory_ReplacesHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(compactSummaryJSON))
	}))
	defer server.Close()

	a := newCompactTestAgent(t, server.URL)
	a.AppendUserMessage("write auth tests")
	a.AppendAssistantMessage("I'll write tests for the auth module.")
	a.AppendUserMessage("add edge cases")
	a.AppendAssistantMessage("Done, added edge cases.")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	summary, err := a.CompactHistory(ctx)
	require.NoError(t, err)
	assert.Contains(t, summary, "auth tests")
	assert.Equal(t, 2, a.HistoryLen(), "history should be replaced with 2 messages")
}

func TestCompactHistory_TooShortHistory(t *testing.T) {
	a := newCompactTestAgent(t, "http://unused")

	ctx := context.Background()
	_, err := a.CompactHistory(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fewer than 2 messages")
}

func TestCompactHistory_SingleMessage(t *testing.T) {
	a := newCompactTestAgent(t, "http://unused")
	a.AppendUserMessage("hello")

	ctx := context.Background()
	_, err := a.CompactHistory(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fewer than 2 messages")
}

func TestAutoCompactThreshold_IsReasonable(t *testing.T) {
	// 80% of 200k context window
	assert.Equal(t, 160_000, agent.AutoCompactThreshold)
}

func TestCompactHistory_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"type": "server_error", "message": "internal error"}}`))
	}))
	defer server.Close()

	a := newCompactTestAgent(t, server.URL)
	a.AppendUserMessage("hello")
	a.AppendAssistantMessage("hi")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := a.CompactHistory(ctx)
	assert.Error(t, err)
	// History should be unchanged on error
	assert.Equal(t, 2, a.HistoryLen())
}
