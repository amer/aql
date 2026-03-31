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

// TestStreamingReplay_IncrementalDeltas verifies that the runner emits
// individual text deltas as they arrive from the streaming API, not one
// big chunk at the end. This is what makes the token counter tick up
// in real-time.
func TestStreamingReplay_IncrementalDeltas(t *testing.T) {
	fixture, err := os.ReadFile("testdata/stream_hello.sse")
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
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

	var texts []string
	var gotDone bool

	for evt := range ch {
		require.NoError(t, evt.Error)
		if evt.Done {
			gotDone = true
			break
		}
		if evt.Text != "" {
			texts = append(texts, evt.Text)
		}
	}

	assert.True(t, gotDone, "should receive Done event")
	// The SSE fixture has 2 separate text deltas: "hello" and " world"
	// With real streaming, each delta should arrive as a separate event
	assert.GreaterOrEqual(t, len(texts), 2,
		"should receive multiple incremental text events, got %d: %v", len(texts), texts)
	assert.Equal(t, "hello", texts[0])
	assert.Equal(t, " world", texts[1])
}

// TestStreamingReplay_ToolUse verifies streaming handles tool_use blocks:
// text deltas arrive incrementally, then tool call is emitted, executed,
// and results fed back.
func TestStreamingReplay_ToolUse(t *testing.T) {
	callCount := 0

	sseFixture, err := os.ReadFile("testdata/stream_tool_use.sse")
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call: stream with tool_use
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.Write(sseFixture)
		} else {
			// Second call: final text response (non-streaming OK for simplicity)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_2","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-6","stop_reason":null,"usage":{"input_tokens":50,"output_tokens":1}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Done."}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":2}}

event: message_stop
data: {"type":"message_stop"}

`))
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

	var texts []string
	var toolCalls []string
	var toolDones []string
	var gotDone bool

	for evt := range ch {
		require.NoError(t, evt.Error)
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
	assert.Equal(t, 2, callCount, "should make 2 API calls")

	// Text should arrive incrementally (at least "Let me " and "check that." separately)
	assert.GreaterOrEqual(t, len(texts), 2, "text deltas should arrive incrementally")

	// Tool call should have been detected and executed
	assert.Equal(t, []string{"bash"}, toolCalls)
	require.Len(t, toolDones, 1)
	assert.Contains(t, toolDones[0], "hello")
}
