package agent_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/amer/aql/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunReplaysThinkingBlock verifies that a thinking block emitted alongside a
// tool_use is preserved in the assistant message and replayed on the follow-up
// request. Dropping it makes the API reject the tool_use turn with a 400.
func TestRunReplaysThinkingBlock(t *testing.T) {
	callCount := 0
	var secondBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			serveSSE(w, jsonToSSE([]byte(`{
				"id": "msg_1",
				"type": "message",
				"role": "assistant",
				"content": [
					{"type": "thinking", "thinking": "I should run echo", "signature": "sig-777"},
					{"type": "tool_use", "id": "tu_1", "name": "bash", "input": {"command": "echo hi"}}
				],
				"model": "claude-sonnet-4-6-20250514",
				"stop_reason": "tool_use",
				"usage": {"input_tokens": 30, "output_tokens": 20}
			}`)))
			return
		}
		body, _ := io.ReadAll(r.Body)
		secondBody = string(body)
		serveSSE(w, jsonToSSE([]byte(`{
			"id": "msg_2",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "Done."}],
			"model": "claude-sonnet-4-6-20250514",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 50, "output_tokens": 5}
		}`)))
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
	for evt := range ch {
		require.NoError(t, evt.Error)
		if evt.History != nil {
			coder.ApplyHistory(evt.History.Message)
		}
		if evt.Done {
			break
		}
	}

	require.Equal(t, 2, callCount, "expected a follow-up request after tool use")
	assert.True(t, strings.Contains(secondBody, "I should run echo"),
		"follow-up request must replay the thinking text")
	assert.True(t, strings.Contains(secondBody, "sig-777"),
		"follow-up request must replay the thinking signature")
}
