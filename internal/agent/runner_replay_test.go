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

// TestRunnerReplay_StreamsFromFixture replays a recorded SSE response
// without calling the real API.
func TestRunnerReplay_StreamsFromFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/stream_hello.sse")
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
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

	var chunks []string
	var gotDone bool

	for evt := range ch {
		if evt.Error != nil {
			t.Fatalf("unexpected error: %v", evt.Error)
		}
		if evt.Done {
			gotDone = true
			break
		}
		chunks = append(chunks, evt.Text)
	}

	assert.True(t, gotDone, "should receive Done event")
	require.True(t, len(chunks) > 0, "should receive at least one text chunk")

	var full string
	for _, c := range chunks {
		full += c
	}
	assert.Equal(t, "hello world", full)
	t.Logf("replayed %d chunks, response: %q", len(chunks), full)
}
