package agent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/amer/aql/internal/agent"
	"github.com/amer/aql/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunEmitsHistoryAppend verifies that Run() emits HistoryAppend events
// instead of mutating history internally. The caller must apply them.
func TestRunEmitsHistoryAppend(t *testing.T) {
	fixture, err := os.ReadFile("testdata/message_hello.json")
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveSSE(w, jsonToSSE(fixture))
	}))
	defer server.Close()

	opts := testClientOpts(server.URL)
	coder, err := agent.New(agent.Config{
		Name:         "test",
		Role:         "assistant",
		SystemPrompt: "Reply with hello world.",
	}, t.TempDir(), opts...)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := coder.Run(ctx, "say hello")

	var historyMsgs []domain.Message
	for evt := range ch {
		require.NoError(t, evt.Error)
		if evt.History != nil {
			historyMsgs = append(historyMsgs, evt.History.Message)
			coder.ApplyHistory(evt.History.Message)
		}
		if evt.Done {
			break
		}
	}

	// Should emit at least 2 history messages: the user message and the assistant response
	require.GreaterOrEqual(t, len(historyMsgs), 2, "expected at least user + assistant history events")

	// First should be user message
	assert.Equal(t, domain.RoleUser, historyMsgs[0].Role)
	assert.Equal(t, "say hello", historyMsgs[0].Content[0].Text)

	// Last should be assistant message
	last := historyMsgs[len(historyMsgs)-1]
	assert.Equal(t, domain.RoleAssistant, last.Role)

	// History should have been applied to the agent
	assert.Equal(t, len(historyMsgs), coder.HistoryLen())
}

// TestRunHistoryNotMutatedInternally verifies that if the caller does NOT
// call ApplyHistory, the agent's history stays empty after Run completes.
func TestRunHistoryNotMutatedInternally(t *testing.T) {
	fixture, err := os.ReadFile("testdata/message_hello.json")
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveSSE(w, jsonToSSE(fixture))
	}))
	defer server.Close()

	opts := testClientOpts(server.URL)
	coder, err := agent.New(agent.Config{
		Name:         "test",
		Role:         "assistant",
		SystemPrompt: "Reply with hello world.",
	}, t.TempDir(), opts...)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := coder.Run(ctx, "say hello")

	// Drain events but do NOT apply history
	for evt := range ch {
		if evt.Done || evt.Error != nil {
			break
		}
	}

	// Agent's internal history should be empty — Run() didn't mutate it
	assert.Equal(t, 0, coder.HistoryLen(),
		"Run() should not mutate history internally; caller must apply via ApplyHistory")
}

// TestRunToolUseEmitsHistoryForToolResults verifies that tool use loops
// emit history events for the assistant response and tool result messages.
func TestRunToolUseEmitsHistoryForToolResults(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			serveSSE(w, jsonToSSE([]byte(`{
				"id": "msg_1",
				"type": "message",
				"role": "assistant",
				"content": [
					{"type": "text", "text": "Let me check."},
					{"type": "tool_use", "id": "tu_1", "name": "bash", "input": {"command": "echo hi"}}
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
				"content": [{"type": "text", "text": "Done."}],
				"model": "claude-sonnet-4-6-20250514",
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 50, "output_tokens": 5}
			}`)))
		}
	}))
	defer server.Close()

	opts := testClientOpts(server.URL)
	coder, err := agent.New(agent.Config{
		Name:         "test",
		Role:         "assistant",
		SystemPrompt: "Use tools.",
	}, t.TempDir(), opts...)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := coder.Run(ctx, "run echo")

	var historyMsgs []domain.Message
	for evt := range ch {
		require.NoError(t, evt.Error)
		if evt.History != nil {
			historyMsgs = append(historyMsgs, evt.History.Message)
			coder.ApplyHistory(evt.History.Message)
		}
		if evt.Done {
			break
		}
	}

	// Expected history: user msg, assistant (tool_use), tool results (user), assistant (end_turn)
	require.Equal(t, 4, len(historyMsgs), "expected 4 history messages for tool use flow")
	assert.Equal(t, domain.RoleUser, historyMsgs[0].Role)      // user message
	assert.Equal(t, domain.RoleAssistant, historyMsgs[1].Role) // assistant with tool_use
	assert.Equal(t, domain.RoleUser, historyMsgs[2].Role)      // tool results
	assert.Equal(t, domain.RoleAssistant, historyMsgs[3].Role) // final response
}
