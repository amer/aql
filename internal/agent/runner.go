package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
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

		model := ResolveModel(a.config.Model)
		slog.Debug("starting API stream", "agent", a.config.Name, "model", string(model), "historyLength", len(a.history), "oauth", a.isOAuth)

		params, reqOpts := a.buildMessageParams(model)
		stream := a.client.Messages.NewStreaming(ctx, params, reqOpts...)

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
				Error:     enrichAPIError(err, string(model)),
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

// billingHeader is the Claude Code billing header that enables access to
// Opus/Sonnet models via OAuth Console login. Discovered via mitmproxy
// reverse-engineering of Claude Code's API calls.
const billingHeader = "x-anthropic-billing-header: cc_version=2.1.87.7b6; cc_entrypoint=cli; cch=22c94;"

// claudeCodeBetas are the beta feature flags required for Claude Code billing.
const claudeCodeBetas = "claude-code-20250219,interleaved-thinking-2025-05-14,effort-2025-11-24"

// buildMessageParams constructs the API request params. When the agent uses
// OAuth authentication, it injects the billing header, adaptive thinking,
// and output_config required for Opus/Sonnet access.
func (a *Agent) buildMessageParams(model anthropic.Model) (anthropic.MessageNewParams, []option.RequestOption) {
	system := []anthropic.TextBlockParam{
		{Text: a.systemPrompt},
	}

	params := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 4096,
		System:    system,
		Messages:  a.history,
	}

	var reqOpts []option.RequestOption

	if a.isOAuth {
		// Prepend billing header as first system block (required for Opus access)
		params.System = append(
			[]anthropic.TextBlockParam{{Text: billingHeader}},
			params.System...,
		)

		// Adaptive thinking (required)
		params.Thinking = anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
		}

		// Output config with medium effort (required)
		params.OutputConfig = anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortMedium,
		}

		// MaxTokens must be higher when thinking is enabled
		params.MaxTokens = 16384

		// Beta headers for Claude Code billing
		reqOpts = append(reqOpts, option.WithHeader("anthropic-beta", claudeCodeBetas))

		slog.Debug("injected Claude Code billing header for OAuth",
			"agent", a.config.Name, "model", string(model))
	}

	return params, reqOpts
}

// enrichAPIError adds actionable context to common API errors.
func enrichAPIError(err error, model string) error {
	msg := err.Error()
	if strings.Contains(msg, "400") {
		return fmt.Errorf("%w — your API key may not have access to %s. "+
			"Run `aql auth login --console` for full model access, "+
			"or /model to switch models", err, model)
	}
	if strings.Contains(msg, "404") {
		return fmt.Errorf("%w — model %q not found. Try /model to pick a valid model", err, model)
	}
	return err
}
