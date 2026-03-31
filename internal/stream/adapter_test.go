package stream_test

import (
	"context"
	"sync"
	"testing"

	"github.com/amer/aql/internal/domain"
	"github.com/amer/aql/internal/stream"
	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func collectMessages(ch <-chan domain.StreamEvent) []any {
	var msgs []any
	var mu sync.Mutex

	send := func(msg any) {
		mu.Lock()
		defer mu.Unlock()
		msgs = append(msgs, msg)
	}

	stream.Forward(context.Background(), ch, send)

	mu.Lock()
	defer mu.Unlock()
	return msgs
}

func TestForward_TextDelta(t *testing.T) {
	ch := make(chan domain.StreamEvent, 2)
	ch <- domain.StreamEvent{AgentName: "coder", Text: "hello"}
	ch <- domain.StreamEvent{AgentName: "coder", Done: true}
	close(ch)

	msgs := collectMessages(ch)
	require.Len(t, msgs, 2)
	assert.Equal(t, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "hello"}, msgs[0])
	assert.Equal(t, tui.AgentStreamDoneMsg{AgentName: "coder"}, msgs[1])
}

func TestForward_Error(t *testing.T) {
	ch := make(chan domain.StreamEvent, 1)
	ch <- domain.StreamEvent{AgentName: "coder", Error: assert.AnError}
	close(ch)

	msgs := collectMessages(ch)
	require.Len(t, msgs, 1)
	errMsg, ok := msgs[0].(tui.AgentStreamErrorMsg)
	require.True(t, ok)
	assert.Equal(t, "coder", errMsg.AgentName)
	assert.Equal(t, assert.AnError, errMsg.Error)
}

func TestForward_ToolCallAndDone(t *testing.T) {
	ch := make(chan domain.StreamEvent, 3)
	ch <- domain.StreamEvent{
		AgentName: "coder",
		ToolCall: &domain.ToolCallEvent{
			ToolName: "bash",
			ToolID:   "tu_1",
			Input:    `{"command":"echo hi"}`,
		},
	}
	ch <- domain.StreamEvent{
		AgentName: "coder",
		ToolDone: &domain.ToolDoneEvent{
			ToolName: "bash",
			ToolID:   "tu_1",
			Output:   "hi\n",
		},
	}
	ch <- domain.StreamEvent{AgentName: "coder", Done: true}
	close(ch)

	msgs := collectMessages(ch)
	require.Len(t, msgs, 3)

	// ToolCall
	toolCall, ok := msgs[0].(tui.AgentToolCallMsg)
	require.True(t, ok)
	assert.Equal(t, "bash", toolCall.ToolCall.Name)
	assert.Equal(t, domain.ToolRunning, toolCall.ToolCall.Status)

	// ToolDone
	toolDone, ok := msgs[1].(tui.AgentToolCallMsg)
	require.True(t, ok)
	assert.Equal(t, "bash", toolDone.ToolCall.Name)
	assert.Equal(t, domain.ToolDone, toolDone.ToolCall.Status)
	assert.Equal(t, "hi\n", toolDone.ToolCall.Content)
}

func TestForward_SkipsAskUser(t *testing.T) {
	ch := make(chan domain.StreamEvent, 3)
	ch <- domain.StreamEvent{
		AgentName: "coder",
		ToolCall:  &domain.ToolCallEvent{ToolName: "ask_user", ToolID: "tu_1"},
	}
	ch <- domain.StreamEvent{
		AgentName: "coder",
		ToolDone:  &domain.ToolDoneEvent{ToolName: "ask_user", ToolID: "tu_1", Output: "answer"},
	}
	ch <- domain.StreamEvent{AgentName: "coder", Done: true}
	close(ch)

	msgs := collectMessages(ch)
	// Only the Done message should come through — ask_user events are skipped
	require.Len(t, msgs, 1)
	assert.IsType(t, tui.AgentStreamDoneMsg{}, msgs[0])
}

func TestForward_TokenUsage(t *testing.T) {
	ch := make(chan domain.StreamEvent, 2)
	ch <- domain.StreamEvent{
		AgentName:  "coder",
		TokenUsage: &domain.TokenUsageEvent{InputTokens: 100, OutputTokens: 50},
	}
	ch <- domain.StreamEvent{AgentName: "coder", Done: true}
	close(ch)

	msgs := collectMessages(ch)
	require.Len(t, msgs, 2)
	usage, ok := msgs[0].(tui.TokenUsageMsg)
	require.True(t, ok)
	assert.Equal(t, 100, usage.InputTokens)
	assert.Equal(t, 50, usage.OutputTokens)
}

func TestForward_ToolError(t *testing.T) {
	ch := make(chan domain.StreamEvent, 2)
	ch <- domain.StreamEvent{
		AgentName: "coder",
		ToolDone: &domain.ToolDoneEvent{
			ToolName: "bash",
			ToolID:   "tu_1",
			Output:   "command not found",
			IsError:  true,
		},
	}
	ch <- domain.StreamEvent{AgentName: "coder", Done: true}
	close(ch)

	msgs := collectMessages(ch)
	require.Len(t, msgs, 2)

	toolMsg, ok := msgs[0].(tui.AgentToolCallMsg)
	require.True(t, ok)
	assert.Equal(t, domain.ToolError, toolMsg.ToolCall.Status)
}

func TestForward_ContextCancelled(t *testing.T) {
	ch := make(chan domain.StreamEvent) // unbuffered, will block

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	var msgs []any
	send := func(msg any) {
		msgs = append(msgs, msg)
	}

	stream.Forward(ctx, ch, send)
	assert.Empty(t, msgs, "should return immediately on cancelled context")
}
