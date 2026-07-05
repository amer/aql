package agent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amer/aql/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAutoCompact_DoesNotEmitFabricatedTokenUsage verifies that after an
// auto-compaction the runner emits the history Replace event but does NOT
// follow it with a fabricated token-usage estimate. A made-up count
// (len(summary)/4) would be shown to the user as if it were a real measured
// value; the next real API call reports the accurate count instead.
func TestAutoCompact_DoesNotEmitFabricatedTokenUsage(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// Streaming turn: input tokens (reported in message_delta) exceed
			// the auto-compact threshold.
			serveSSE(w, []byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-6","stop_reason":null,"usage":{"input_tokens":200000,"output_tokens":1}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"done"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"input_tokens":200000,"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`))
			return
		}
		// Compaction SendMessage call (non-streaming JSON).
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "msg_compact",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "A concise summary of the prior conversation."}],
			"model": "claude-sonnet-4-6-20250514",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 100, "output_tokens": 20}
		}`))
	}))
	defer server.Close()

	opts := testClientOpts(server.URL)
	coder, err := agent.New(agent.Config{Name: "test", Role: "assistant", SystemPrompt: "test"}, t.TempDir(), opts...)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := coder.Run(ctx, "go")

	var kinds []string
	for evt := range ch {
		require.NoError(t, evt.Error)
		switch {
		case evt.Replace != nil:
			kinds = append(kinds, "replace")
		case evt.TokenUsage != nil:
			kinds = append(kinds, "token")
		case evt.Done:
			kinds = append(kinds, "done")
		}
		if evt.Done {
			break
		}
	}

	require.Contains(t, kinds, "replace", "auto-compaction should emit a Replace event")

	// No token-usage event may appear after the Replace — that would be the
	// fabricated post-compaction estimate.
	replaceIdx := -1
	for i, k := range kinds {
		if k == "replace" {
			replaceIdx = i
		}
	}
	for _, k := range kinds[replaceIdx+1:] {
		assert.NotEqual(t, "token", k, "no fabricated token-usage event may follow compaction")
	}
}
