package agent

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
)

// StreamEvent represents a chunk of output from the agent.
type StreamEvent struct {
	AgentName string
	Text      string
	Done      bool
	Error     error
}

// Run sends a user message to Claude and streams responses back on the
// returned channel. This is the imperative shell — it handles I/O only.
func (a *Agent) Run(ctx context.Context, userMessage string) <-chan StreamEvent {
	ch := make(chan StreamEvent, 64)

	go func() {
		defer close(ch)

		a.history = append(a.history, anthropic.NewUserMessage(
			anthropic.NewTextBlock(userMessage),
		))

		stream := a.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
			Model:     anthropic.ModelClaudeSonnet4_20250514,
			MaxTokens: 4096,
			System: []anthropic.TextBlockParam{
				{Text: a.systemPrompt},
			},
			Messages: a.history,
		})

		var fullResponse string
		for stream.Next() {
			evt := stream.Current()
			switch variant := evt.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				switch delta := variant.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					fullResponse += delta.Text
					ch <- StreamEvent{
						AgentName: a.config.Name,
						Text:      delta.Text,
					}
				}
			}
		}

		if err := stream.Err(); err != nil {
			ch <- StreamEvent{
				AgentName: a.config.Name,
				Error:     err,
			}
			return
		}

		// Add assistant response to history
		a.history = append(a.history, anthropic.NewAssistantMessage(
			anthropic.NewTextBlock(fullResponse),
		))

		ch <- StreamEvent{
			AgentName: a.config.Name,
			Done:      true,
		}
	}()

	return ch
}
