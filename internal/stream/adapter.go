package stream

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Forward() — translates domain.StreamEvent → TUI messages
//     (without history)
//   - ForwardWithHistory() — same but applies history mutations
//     via callbacks
//   - drain/drainHistory — cancellation-path drainers that keep the
//     producer from blocking on a full buffer
//   - applyHistory — applies a single event's history mutation
//   - forwardEvent — single event translation
//   - forwardToolCall/forwardToolDone — tool event mapping
//   - SendFunc/HistoryCallbacks types
//
// MUST NOT GO HERE:
//   - Agent logic, tool execution, TUI rendering
//   - Event filtering (except ask_user suppression — that's the
//     one documented exception)
//   - This is a dumb translation pipe.
//
// Q: Should I filter out a new tool type here?
// A: Probably not. Filtering belongs in tui/handlers.go. The only
//    exception is ask_user because it has its own message path.
//
// Q: Should I add a new TUI message type for a new event?
// A: Add the field to domain.StreamEvent, handle it in
//    forwardEvent() here, define the TUI message in tui/types.go,
//    handle it in tui/handlers.go.
//
// Q: Who calls Forward vs ForwardWithHistory?
// A: ForwardWithHistory is used in production (main.go). Forward
//    is the simpler version without history tracking.
// ──────────────────────────────────────────────────────────────────

import (
	"context"

	"github.com/amer/aql/internal/domain"
	"github.com/amer/aql/internal/tui"
)

// SendFunc sends a message to the TUI program.
type SendFunc func(msg any)

// Forward reads domain.StreamEvents from ch and translates them into
// TUI messages via send. It blocks until the channel is closed or a terminal
// event (Done/Error) is received. On cancellation it stops forwarding but keeps
// draining ch until the producer closes it, so the producer never blocks on a
// full buffer (see drain).
func Forward(ctx context.Context, ch <-chan domain.StreamEvent, send SendFunc) {
	for {
		select {
		case <-ctx.Done():
			drain(ch)
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

// drain consumes the remaining events on ch until it closes, discarding them.
// The agent's Run goroutine sends unconditionally on a buffered channel; if the
// consumer stopped reading on cancellation, a full buffer would block that
// goroutine forever, leaking it and the open HTTP stream.
func drain(ch <-chan domain.StreamEvent) {
	for range ch {
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
			// Cancelled (e.g. the user pressed Esc). The TUI has already reset
			// its own streaming state, so forwarding further events would
			// restart it. Keep draining and applying history mutations until the
			// producer closes: draining stops the Run goroutine blocking on a
			// full buffer, and applying history prevents a dropped tool_result
			// from leaving a dangling tool_use that the API rejects next turn.
			drainHistory(ch, history)
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			applyHistory(evt, history)
			if forwardEvent(evt, send) {
				return
			}
		}
	}
}

// applyHistory applies any history mutation carried by evt via the callbacks.
func applyHistory(evt domain.StreamEvent, history HistoryCallbacks) {
	if evt.History != nil && history.Append != nil {
		history.Append(evt.History.Message)
	}
	if evt.Replace != nil && history.Replace != nil {
		history.Replace(evt.Replace.Messages)
	}
}

// drainHistory consumes ch until it closes, applying every history mutation but
// forwarding nothing to the TUI. Used on the cancellation path so history stays
// consistent while the stream is torn down.
func drainHistory(ch <-chan domain.StreamEvent, history HistoryCallbacks) {
	for evt := range ch {
		applyHistory(evt, history)
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
	case evt.StreamReset:
		send(tui.AgentStreamResetMsg{AgentName: evt.AgentName})
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
