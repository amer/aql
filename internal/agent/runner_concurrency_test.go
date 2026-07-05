package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/amer/aql/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunnerBoundsToolParallelism verifies that when the model requests more
// tool calls than the concurrency cap, the runner never runs more than
// maxToolConcurrency tools at once — protecting against unbounded goroutine
// fan-out and resource exhaustion — while still executing every tool.
func TestRunnerBoundsToolParallelism(t *testing.T) {
	const toolCount = 12

	var current, max atomic.Int32
	var executed atomic.Int32
	mockExecutor := func(ctx context.Context, workDir, name string, input json.RawMessage) (string, error) {
		executed.Add(1)
		cur := current.Add(1)
		for {
			old := max.Load()
			if cur <= old || max.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		current.Add(-1)
		return "ok", nil
	}

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			serveSSE(w, jsonToSSE([]byte(manyToolUsesJSON(toolCount))))
			return
		}
		serveSSE(w, jsonToSSE([]byte(`{
			"id": "msg_2",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "done"}],
			"model": "claude-sonnet-4-6-20250514",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 50, "output_tokens": 5}
		}`)))
	}))
	defer server.Close()

	opts := append(testClientOpts(server.URL), agent.WithToolExecutor(mockExecutor))
	coder, err := agent.New(agent.Config{Name: "test", Role: "assistant", SystemPrompt: "test"}, t.TempDir(), opts...)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := coder.Run(ctx, "go")
	for evt := range ch {
		require.NoError(t, evt.Error)
		if evt.Done {
			break
		}
	}

	assert.Equal(t, int32(toolCount), executed.Load(), "every tool must still execute")
	assert.LessOrEqual(t, max.Load(), int32(agent.MaxToolConcurrency),
		"concurrent tool executions must not exceed the cap (got %d)", max.Load())
}

// manyToolUsesJSON builds a Messages API response with n tool_use blocks.
func manyToolUsesJSON(n int) string {
	var b strings.Builder
	b.WriteString(`{"id":"msg_1","type":"message","role":"assistant","content":[`)
	for i := range n {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"type":"tool_use","id":"tu_%d","name":"read_file","input":{"path":"p%d"}}`, i, i)
	}
	b.WriteString(`],"model":"claude-sonnet-4-6-20250514","stop_reason":"tool_use","usage":{"input_tokens":30,"output_tokens":20}}`)
	return b.String()
}
