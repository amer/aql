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
			if evt.Error != nil {
				send(tui.AgentStreamErrorMsg{
					AgentName: evt.AgentName,
					Error:     evt.Error,
				})
				return
			}
			if evt.Done {
				send(tui.AgentStreamDoneMsg{
					AgentName: evt.AgentName,
				})
				return
			}
			if evt.ToolCall != nil {
				// ask_user is displayed via AgentAskUserMsg, not as a tool block
				if evt.ToolCall.ToolName != "ask_user" {
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
				continue
			}
			if evt.ToolDone != nil {
				// ask_user results are already shown inline
				if evt.ToolDone.ToolName == "ask_user" {
					continue
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
				continue
			}
			if evt.TokenUsage != nil {
				send(tui.TokenUsageMsg{
					InputTokens:  evt.TokenUsage.InputTokens,
					OutputTokens: evt.TokenUsage.OutputTokens,
				})
				continue
			}
			if evt.Text != "" {
				send(tui.AgentStreamDeltaMsg{
					AgentName: evt.AgentName,
					Delta:     evt.Text,
				})
			}
		}
	}
}
