package stream

import (
	"context"

	"github.com/amer/aql/internal/domain"
	"github.com/amer/aql/internal/tui"
)

// SendFunc sends a message to the TUI program.
type SendFunc func(msg any)

// Forward reads domain.StreamEvents from ch and translates them into
// TUI messages via send. It blocks until the channel is closed, the
// context is cancelled, or a terminal event (Done/Error) is received.
func Forward(ctx context.Context, ch <-chan domain.StreamEvent, send SendFunc) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			if forwardEvent(evt, send) {
				return
			}
		}
	}
}

// HistoryApplier applies a history message to the agent.
// Provided by the caller so the adapter doesn't import the agent package.
type HistoryApplier func(msg domain.Message)

// HistoryReplacer replaces the agent's entire history (for compaction).
type HistoryReplacer func(msgs []domain.Message)

// HistoryCallbacks holds the functions for applying history mutations.
type HistoryCallbacks struct {
	Append  HistoryApplier
	Replace HistoryReplacer
}

// ForwardWithHistory reads domain.StreamEvents and translates them into TUI
// messages, applying history events via the provided callbacks. This keeps
// history mutation in the Forward goroutine (single writer), not inside
// the agent's Run goroutine.
func ForwardWithHistory(ctx context.Context, ch <-chan domain.StreamEvent, send SendFunc, history HistoryCallbacks) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			if evt.History != nil && history.Append != nil {
				history.Append(evt.History.Message)
			}
			if evt.Replace != nil && history.Replace != nil {
				history.Replace(evt.Replace.Messages)
			}
			if forwardEvent(evt, send) {
				return
			}
		}
	}
}

// forwardEvent translates a single StreamEvent into a TUI message.
// Returns true if the event is terminal (done or error).
func forwardEvent(evt domain.StreamEvent, send SendFunc) bool {
	switch {
	case evt.Error != nil:
		send(tui.AgentStreamErrorMsg{AgentName: evt.AgentName, Error: evt.Error})
		return true
	case evt.Done:
		send(tui.AgentStreamDoneMsg{AgentName: evt.AgentName})
		return true
	case evt.ToolCall != nil:
		forwardToolCall(evt, send)
	case evt.ToolDone != nil:
		forwardToolDone(evt, send)
	case evt.TokenUsage != nil:
		send(tui.TokenUsageMsg{
			InputTokens:  evt.TokenUsage.InputTokens,
			OutputTokens: evt.TokenUsage.OutputTokens,
		})
	case evt.Text != "":
		send(tui.AgentStreamDeltaMsg{AgentName: evt.AgentName, Delta: evt.Text})
	case evt.History != nil:
		// History events are handled by ForwardWithHistory; Forward ignores them.
	}
	return false
}

func forwardToolCall(evt domain.StreamEvent, send SendFunc) {
	if evt.ToolCall.ToolName == "ask_user" {
		return
	}
	send(tui.AgentToolCallMsg{
		AgentName: evt.AgentName,
		ToolCall: domain.ToolCall{
			Name:    evt.ToolCall.ToolName,
			Content: evt.ToolCall.Input,
			Status:  domain.ToolRunning,
			ToolID:  evt.ToolCall.ToolID,
		},
	})
}

func forwardToolDone(evt domain.StreamEvent, send SendFunc) {
	if evt.ToolDone.ToolName == "ask_user" {
		return
	}
	status := domain.ToolDone
	if evt.ToolDone.IsError {
		status = domain.ToolError
	}
	send(tui.AgentToolCallMsg{
		AgentName: evt.AgentName,
		ToolCall: domain.ToolCall{
			Name:    evt.ToolDone.ToolName,
			Content: evt.ToolDone.Output,
			Status:  status,
			ToolID:  evt.ToolDone.ToolID,
		},
	})
}
