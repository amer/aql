package agent_test

import (
	"context"
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

// TestRunner_StreamResetOnMidStreamRetry reproduces H3: when the first
// streaming attempt emits partial text and then fails with a retryable error,
// the runner re-invokes StreamMessage and the retry re-emits the full text.
// Without a reset signal the caller concatenates the partial first attempt with
// the full retry ("partiapartial answer"). The runner must emit a StreamReset
// event before retrying so the caller can discard the partial text.
func TestRunner_StreamResetOnMidStreamRetry(t *testing.T) {
	helloFixture, err := os.ReadFile("testdata/stream_hello.sse")
	require.NoError(t, err)

	// First attempt: stream a partial text delta, then fail with a retryable
	// api_error mid-stream.
	partialThenError := "event: message_start\n" +
		`data: {"type":"message_start","message":{"id":"msg_partial","type":"message","role":"assistant","content":[],"model":"claude-haiku-4-5","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":25,"output_tokens":1}}}` + "\n\n" +
		"event: content_block_start\n" +
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n" +
		"event: content_block_delta\n" +
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"partial answer"}}` + "\n\n" +
		"event: error\n" +
		`data: {"type":"error","error":{"details":null,"type":"api_error","message":"Internal server error"},"request_id":"req_partial"}` + "\n\n"

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if n == 1 {
			w.Write([]byte(partialThenError))
			return
		}
		w.Write(helloFixture)
	}))
	defer server.Close()

	coder, err := agent.New(agent.Config{
		Name:         "test",
		Role:         "assistant",
		SystemPrompt: "Reply with hello world.",
	}, t.TempDir(), testClientOpts(server.URL)...)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ch := coder.Run(ctx, "say hello")

	// Record the ordered sequence of text and reset events.
	var seq []string
	for evt := range ch {
		require.NoError(t, evt.Error, "should not surface transient error after retry succeeds")
		if evt.StreamReset {
			seq = append(seq, "reset")
		}
		if evt.Text != "" {
			seq = append(seq, "text:"+evt.Text)
		}
		if evt.Done {
			break
		}
	}

	require.Equal(t, int32(2), callCount.Load(), "should have made 2 API calls (1 retry)")
	require.Contains(t, seq, "reset", "runner must emit a StreamReset before the retry")

	resetIdx := -1
	for i, s := range seq {
		if s == "reset" {
			resetIdx = i
			break
		}
	}

	// The partial text must precede the reset; the retry's text must follow it.
	assert.Contains(t, seq[:resetIdx], "text:partial answer", "partial text should precede the reset")
	foundAfter := false
	for _, s := range seq[resetIdx+1:] {
		if s == "text:hello" {
			foundAfter = true
		}
	}
	assert.True(t, foundAfter, "retry text should follow the reset; got %v", seq)
}
