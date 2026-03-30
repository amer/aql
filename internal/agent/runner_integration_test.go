package agent_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/amer/aql/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunnerLive_StreamsFromAPI calls the real Claude API.
// Only runs when AQL_LIVE_TEST=1 is set — use this to validate the cache
// or re-record fixtures.
func TestRunnerLive_StreamsFromAPI(t *testing.T) {
	if os.Getenv("AQL_LIVE_TEST") != "1" {
		t.Skip("set AQL_LIVE_TEST=1 to run live API tests")
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	workDir := t.TempDir()

	coder, err := agent.New(agent.Config{
		Name:         "test-coder",
		Role:         "Go developer",
		SystemPrompt: "Reply with exactly: hello world. Nothing else.",
	}, workDir)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch := coder.Run(ctx, "say hello")

	var chunks []string
	var gotDone bool
	var gotError error

	for evt := range ch {
		if evt.Error != nil {
			gotError = evt.Error
			break
		}
		if evt.Done {
			gotDone = true
			break
		}
		chunks = append(chunks, evt.Text)
		assert.Equal(t, "test-coder", evt.AgentName)
	}

	require.NoError(t, gotError, "API should not return an error")
	assert.True(t, gotDone, "should receive Done event")
	assert.True(t, len(chunks) > 0, "should receive at least one text chunk")

	var full string
	for _, c := range chunks {
		full += c
	}
	assert.Contains(t, full, "hello")
	t.Logf("received %d chunks, full response: %q", len(chunks), full)
}
