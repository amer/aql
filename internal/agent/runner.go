package agent

import (
	"context"
	"log/slog"
	"time"

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

		slog.Debug("agent run started", "agent", a.config.Name, "messageLength", len(userMessage))
		start := time.Now()

		a.history = append(a.history, anthropic.NewUserMessage(
			anthropic.NewTextBlock(userMessage),
		))

		slog.Debug("starting API stream", "agent", a.config.Name, "model", string(anthropic.ModelClaudeSonnet4_20250514), "historyLength", len(a.history))

		stream := a.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
			Model:     anthropic.ModelClaudeSonnet4_20250514,
			MaxTokens: 4096,
			System: []anthropic.TextBlockParam{
				{Text: a.systemPrompt},
			},
			Messages: a.history,
		})

		var fullResponse string
		chunks := 0
		for stream.Next() {
			evt := stream.Current()
			switch variant := evt.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				switch delta := variant.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					fullResponse += delta.Text
					chunks++
					ch <- StreamEvent{
						AgentName: a.config.Name,
						Text:      delta.Text,
					}
				}
			}
		}

		if err := stream.Err(); err != nil {
			slog.Error("API stream error", "agent", a.config.Name, "error", err, "duration", time.Since(start), "chunksReceived", chunks)
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

		slog.Info("agent run completed", "agent", a.config.Name, "duration", time.Since(start), "responseLength", len(fullResponse), "chunks", chunks)

		ch <- StreamEvent{
			AgentName: a.config.Name,
			Done:      true,
		}
	}()

	return ch
}
