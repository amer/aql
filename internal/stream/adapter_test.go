package stream_test

import (
	"context"
	"sync"
	"testing"
	"time"

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

func TestForward_StreamReset(t *testing.T) {
	ch := make(chan domain.StreamEvent, 3)
	ch <- domain.StreamEvent{AgentName: "coder", Text: "partial"}
	ch <- domain.StreamEvent{AgentName: "coder", StreamReset: true}
	ch <- domain.StreamEvent{AgentName: "coder", Done: true}
	close(ch)

	msgs := collectMessages(ch)
	require.Len(t, msgs, 3)
	assert.Equal(t, tui.AgentStreamResetMsg{AgentName: "coder"}, msgs[1])
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

func TestForward_ContextCancelledDrainsAndReturns(t *testing.T) {
	// On cancellation Forward must keep draining the channel until the producer
	// closes it — otherwise the producer blocks forever on a full buffer (C1).
	// Buffered + pre-closed so this is deterministic: the test asserts Forward
	// returns rather than hanging.
	ch := make(chan domain.StreamEvent, 2)
	ch <- domain.StreamEvent{AgentName: "coder", Text: "late delta"}
	ch <- domain.StreamEvent{AgentName: "coder", Done: true}
	close(ch)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	done := make(chan struct{})
	go func() {
		stream.Forward(ctx, ch, func(any) {})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Forward did not return after cancellation — channel was not drained")
	}
}

func TestForwardWithHistory_CancelledStillAppliesHistory(t *testing.T) {
	// C2: on cancellation the assistant tool_use message may already be applied
	// while its tool_result is still in flight. Dropping the tool_result leaves
	// a dangling tool_use that the API rejects. ForwardWithHistory must apply
	// every history event even when the context is cancelled.
	assistantToolUse := domain.Message{
		Role:    domain.RoleAssistant,
		Content: []domain.ContentBlock{domain.ToolUseContentBlock("t1", "bash", `{}`)},
	}
	toolResult := domain.Message{
		Role:    domain.RoleUser,
		Content: []domain.ContentBlock{domain.ToolResultContentBlock("t1", "ok", false)},
	}

	ch := make(chan domain.StreamEvent, 3)
	ch <- domain.StreamEvent{History: &domain.HistoryAppendMsg{Message: assistantToolUse}}
	ch <- domain.StreamEvent{History: &domain.HistoryAppendMsg{Message: toolResult}}
	ch <- domain.StreamEvent{Done: true}
	close(ch)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var applied []domain.Message
	stream.ForwardWithHistory(ctx, ch, func(any) {}, stream.HistoryCallbacks{
		Append: func(m domain.Message) { applied = append(applied, m) },
	})

	assert.Len(t, applied, 2, "both history messages must be applied despite cancellation")
}
