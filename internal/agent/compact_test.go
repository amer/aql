package agent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amer/aql/internal/agent"
	"github.com/amer/aql/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatHistoryForCompaction_EmptyHistory(t *testing.T) {
	result := agent.FormatHistoryForCompaction(nil)
	assert.Equal(t, "", result)
}

func TestFormatHistoryForCompaction_BasicConversation(t *testing.T) {
	history := []domain.Message{
		domain.NewUserMessage("Hello"),
		domain.NewAssistantMessage("Hi there!"),
		domain.NewUserMessage("Write tests"),
		domain.NewAssistantMessage("Done."),
	}

	result := agent.FormatHistoryForCompaction(history)
	assert.Contains(t, result, "User: Hello")
	assert.Contains(t, result, "Assistant: Hi there!")
	assert.Contains(t, result, "User: Write tests")
	assert.Contains(t, result, "Assistant: Done.")
}

func TestFormatHistoryForCompaction_WithToolUse(t *testing.T) {
	history := []domain.Message{
		domain.NewUserMessage("Read main.go"),
		{
			Role: domain.RoleAssistant,
			Content: []domain.ContentBlock{
				domain.ToolUseContentBlock("tool_1", "read_file", `{"path": "main.go"}`),
			},
		},
		{
			Role: domain.RoleUser,
			Content: []domain.ContentBlock{
				domain.ToolResultContentBlock("tool_1", "package main\nfunc main() {}", false),
			},
		},
		domain.NewAssistantMessage("Here is main.go"),
	}

	result := agent.FormatHistoryForCompaction(history)
	assert.Contains(t, result, "User: Read main.go")
	assert.Contains(t, result, "[Tool: read_file]")
	assert.Contains(t, result, "[Tool Result]")
	assert.Contains(t, result, "Assistant: Here is main.go")
}

func TestFormatHistoryForCompaction_MultipleBlocksPerMessage(t *testing.T) {
	history := []domain.Message{
		{
			Role: domain.RoleAssistant,
			Content: []domain.ContentBlock{
				domain.TextBlock("I'll read the file."),
				domain.ToolUseContentBlock("tool_1", "read_file", `{"path": "go.mod"}`),
			},
		},
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
	opts := testClientOpts(serverURL)
	a, err := agent.New(agent.Config{
		Name:         "test-coder",
		Role:         "Go developer",
		SystemPrompt: "You are a Go developer.",
	}, t.TempDir(), opts...)
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
	a.ApplyHistory(domain.NewUserMessage("write auth tests"))
	a.ApplyHistory(domain.NewAssistantMessage("I'll write tests for the auth module."))
	a.ApplyHistory(domain.NewUserMessage("add edge cases"))
	a.ApplyHistory(domain.NewAssistantMessage("Done, added edge cases."))

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
	a.ApplyHistory(domain.NewUserMessage("hello"))

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
	a.ApplyHistory(domain.NewUserMessage("hello"))
	a.ApplyHistory(domain.NewAssistantMessage("hi"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := a.CompactHistory(ctx)
	assert.Error(t, err)
	// History should be unchanged on error
	assert.Equal(t, 2, a.HistoryLen())
}
