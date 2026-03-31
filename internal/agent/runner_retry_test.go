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

// TestRunner_RetryOnTransient500 verifies that the runner retries when
// the API returns a transient 500 error, then succeeds on the next attempt.
func TestRunner_RetryOnTransient500(t *testing.T) {
	helloFixture, err := os.ReadFile("testdata/stream_hello.sse")
	require.NoError(t, err)

	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			// First call: return 500 error via SSE stream
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("event: error\ndata: {\"type\":\"error\",\"error\":{\"details\":null,\"type\":\"api_error\",\"message\":\"Internal server error\"},\"request_id\":\"req_test500\"}\n\n"))
			return
		}
		// Second call: succeed
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(helloFixture)
	}))
	defer server.Close()

	coder, err := agent.New(agent.Config{
		Name:         "test",
		Role:         "assistant",
		SystemPrompt: "Reply with hello world.",
	}, t.TempDir(), agent.WithBaseURL(server.URL), agent.WithAPIKey("test-key"))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ch := coder.Run(ctx, "say hello")

	var texts []string
	var gotDone bool

	for evt := range ch {
		require.NoError(t, evt.Error, "should not surface transient error after retry succeeds")
		if evt.Done {
			gotDone = true
			break
		}
		if evt.Text != "" {
			texts = append(texts, evt.Text)
		}
	}

	assert.True(t, gotDone, "should receive Done event")
	assert.Equal(t, int32(2), callCount.Load(), "should have made 2 API calls (1 retry)")
	assert.Contains(t, texts, "hello")
}

// TestRunner_RetryExhausted verifies that after max retries the error
// is surfaced to the caller.
func TestRunner_RetryExhausted(t *testing.T) {
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		// Always return 500
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("event: error\ndata: {\"type\":\"error\",\"error\":{\"details\":null,\"type\":\"api_error\",\"message\":\"Internal server error\"},\"request_id\":\"req_test500\"}\n\n"))
	}))
	defer server.Close()

	coder, err := agent.New(agent.Config{
		Name:         "test",
		Role:         "assistant",
		SystemPrompt: "test",
	}, t.TempDir(), agent.WithBaseURL(server.URL), agent.WithAPIKey("test-key"))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ch := coder.Run(ctx, "hi")

	var gotError error
	for evt := range ch {
		if evt.Error != nil {
			gotError = evt.Error
			break
		}
	}

	require.Error(t, gotError, "should surface error after retries exhausted")
	assert.Contains(t, gotError.Error(), "Internal server error")
	// Should have retried (1 initial + 2 retries = 3 attempts)
	assert.Equal(t, int32(3), callCount.Load(), "should retry twice before giving up")
}

// TestRunner_NoRetryOn400 verifies that non-transient errors (like 400)
// are NOT retried.
func TestRunner_NoRetryOn400(t *testing.T) {
	fixture, err := os.ReadFile("testdata/error_400.json")
	require.NoError(t, err)

	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(fixture)
	}))
	defer server.Close()

	coder, err := agent.New(agent.Config{
		Name:         "test",
		Role:         "assistant",
		SystemPrompt: "test",
	}, t.TempDir(), agent.WithBaseURL(server.URL), agent.WithAPIKey("test-key"))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := coder.Run(ctx, "hi")

	var gotError error
	for evt := range ch {
		if evt.Error != nil {
			gotError = evt.Error
			break
		}
	}

	require.Error(t, gotError)
	assert.Equal(t, int32(1), callCount.Load(), "should NOT retry on 400")
}
