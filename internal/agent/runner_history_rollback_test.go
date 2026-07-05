package agent_test

import (
	"context"
	"encoding/json"
	"io"
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

// drainApply drains a Run channel, applying history append and replace events
// so the agent's history reflects what the caller (stream forwarder) would do.
func drainApply(t *testing.T, coder *agent.Agent, ch <-chan domain.StreamEvent) {
	t.Helper()
	for evt := range ch {
		if evt.History != nil {
			coder.ApplyHistory(evt.History.Message)
		}
		if evt.Replace != nil {
			coder.ReplaceHistory(evt.Replace.Messages)
		}
	}
}

// TestRunner_FailedTurnRollsBackUserMessage reproduces H2: a turn whose API
// call fails leaves the just-appended user message trailing in history, so the
// next turn appends a second user message and the API rejects two consecutive
// user roles with a 400. The failed turn must roll history back to its
// pre-turn state.
func TestRunner_FailedTurnRollsBackUserMessage(t *testing.T) {
	hello, err := os.ReadFile("testdata/message_hello.json")
	require.NoError(t, err)
	err400, err := os.ReadFile("testdata/error_400.json")
	require.NoError(t, err)

	var callCount int
	var secondReqRoles []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write(err400)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		}
		json.Unmarshal(body, &req)
		for _, m := range req.Messages {
			secondReqRoles = append(secondReqRoles, m.Role)
		}
		serveSSE(w, jsonToSSE(hello))
	}))
	defer server.Close()

	coder, err := agent.New(agent.Config{
		Name:         "test",
		Role:         "assistant",
		SystemPrompt: "test",
		Model:        "claude-opus-4-6",
	}, t.TempDir(), testClientOpts(server.URL)...)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First turn fails at the API. Its user message must not survive in history.
	drainApply(t, coder, coder.Run(ctx, "first question"))

	// Second turn succeeds; capture the roles it sends to the API.
	drainApply(t, coder, coder.Run(ctx, "second question"))

	require.NotEmpty(t, secondReqRoles, "second turn should have reached the API")
	for i := 1; i < len(secondReqRoles); i++ {
		assert.False(t, secondReqRoles[i] == "user" && secondReqRoles[i-1] == "user",
			"two consecutive user roles at %d: %v", i, secondReqRoles)
	}
}
