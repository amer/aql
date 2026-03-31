package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/amer/aql/internal/domain"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const (
	// maxToolIterations is the maximum number of tool use loops before giving up.
	maxToolIterations = 25

	// streamEventBufSize is the buffer size for the domain.StreamEvent channel.
	streamEventBufSize = 64

	// defaultMaxTokens is the max tokens for non-OAuth API requests.
	defaultMaxTokens = 4096

	// oauthMaxTokens is the max tokens for OAuth-authenticated API requests.
	oauthMaxTokens = 16384

	// maxStreamRetries is the number of retry attempts for transient API errors (500, 529).
	maxStreamRetries = 2

	// streamRetryBaseDelay is the base delay between retries (doubled each attempt).
	streamRetryBaseDelay = 500 * time.Millisecond
)

// Run sends a user message to Claude and streams responses back on the
// returned channel. Implements a tool use loop: if Claude responds with
// tool_use blocks, the tools are executed and results sent back until
// Claude produces a final text response.
func (a *Agent) Run(ctx context.Context, userMessage string) <-chan domain.StreamEvent {
	ch := make(chan domain.StreamEvent, streamEventBufSize)

	go func() {
		defer close(ch)

		slog.Debug("agent run started", "agent", a.config.Name, "messageLength", len(userMessage))
		start := time.Now()

		a.history = append(a.history, anthropic.NewUserMessage(
			anthropic.NewTextBlock(userMessage),
		))

		model := ResolveModel(a.config.Model)
		slog.Debug("starting API call", "agent", a.config.Name, "model", string(model), "historyLength", len(a.history), "oauth", a.isOAuth)

		// Tool use loop: keep calling the API until we get end_turn
		for iteration := 0; iteration < maxToolIterations; iteration++ {
			params, reqOpts := a.buildMessageParams(model)

			// Retry loop for transient API errors
			var assistantBlocks []anthropic.ContentBlockParamUnion
			type pendingToolUse struct {
				id       string
				name     string
				inputBuf strings.Builder
			}
			var toolUses []pendingToolUse
			var stopReason anthropic.StopReason
			var inputTokens, outputTokens int64

			streamOK := false
			for attempt := 0; attempt <= maxStreamRetries; attempt++ {
				stream := a.client.Messages.NewStreaming(ctx, params, reqOpts...)

				// Reset accumulators for each attempt
				assistantBlocks = nil
				toolUses = nil
				stopReason = ""
				inputTokens = 0
				outputTokens = 0
				activeBlocks := map[int64]*pendingToolUse{}
				textBlocks := map[int64]*strings.Builder{}

				for stream.Next() {
					evt := stream.Current()

					switch v := evt.AsAny().(type) {
					case anthropic.ContentBlockStartEvent:
						cb := v.ContentBlock
						switch cb.Type {
						case "text":
							textBlocks[v.Index] = &strings.Builder{}
						case "tool_use":
							tu := &pendingToolUse{id: cb.ID, name: cb.Name}
							activeBlocks[v.Index] = tu
							toolUses = append(toolUses, *tu)
						}

					case anthropic.ContentBlockDeltaEvent:
						switch d := v.Delta.AsAny().(type) {
						case anthropic.TextDelta:
							if d.Text != "" {
								ch <- domain.StreamEvent{AgentName: a.config.Name, Text: d.Text}
								if sb, ok := textBlocks[v.Index]; ok {
									sb.WriteString(d.Text)
								}
							}
						case anthropic.InputJSONDelta:
							if tu, ok := activeBlocks[v.Index]; ok {
								tu.inputBuf.WriteString(d.PartialJSON)
								// Update the last toolUses entry for this block
								for i := range toolUses {
									if toolUses[i].id == tu.id {
										toolUses[i].inputBuf = tu.inputBuf
									}
								}
							}
						}

					case anthropic.MessageDeltaEvent:
						stopReason = v.Delta.StopReason
						if v.Usage.OutputTokens > 0 {
							outputTokens = v.Usage.OutputTokens
						}
						if v.Usage.InputTokens > 0 {
							inputTokens = v.Usage.InputTokens
						}
					}
				}

				streamErr := stream.Err()
				stream.Close()

				if streamErr == nil {
					// Build text blocks from this attempt
					textIndices := make([]int64, 0, len(textBlocks))
					for idx := range textBlocks {
						textIndices = append(textIndices, idx)
					}
					sort.Slice(textIndices, func(i, j int) bool { return textIndices[i] < textIndices[j] })
					for _, idx := range textIndices {
						text := textBlocks[idx].String()
						if text != "" {
							assistantBlocks = append(assistantBlocks, anthropic.NewTextBlock(text))
						}
					}
					streamOK = true
					break
				}

				// Check if error is retryable (transient server errors)
				if !isRetryableError(streamErr) || attempt == maxStreamRetries {
					slog.Error("API error", "agent", a.config.Name, "error", streamErr, "duration", time.Since(start), "iteration", iteration, "attempt", attempt)
					ch <- domain.StreamEvent{
						AgentName: a.config.Name,
						Error:     enrichAPIError(streamErr, string(model)),
					}
					return
				}

				delay := streamRetryBaseDelay * time.Duration(1<<attempt)
				slog.Warn("transient API error, retrying",
					"agent", a.config.Name, "error", streamErr, "attempt", attempt+1, "maxRetries", maxStreamRetries, "delay", delay)

				select {
				case <-time.After(delay):
				case <-ctx.Done():
					ch <- domain.StreamEvent{AgentName: a.config.Name, Error: ctx.Err()}
					return
				}
			}

			if !streamOK {
				return
			}

			// Emit precise token counts from this API call
			if inputTokens > 0 || outputTokens > 0 {
				ch <- domain.StreamEvent{
					AgentName: a.config.Name,
					TokenUsage: &domain.TokenUsageEvent{
						InputTokens:  int(inputTokens),
						OutputTokens: int(outputTokens),
					},
				}
				slog.Debug("token usage", "agent", a.config.Name, "input", inputTokens, "output", outputTokens)
			}

			// Append tool_use blocks to assistant message
			for _, tu := range toolUses {
				var input json.RawMessage
				if tu.inputBuf.Len() > 0 {
					input = json.RawMessage(tu.inputBuf.String())
				} else {
					input = json.RawMessage("{}")
				}
				assistantBlocks = append(assistantBlocks, anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID:    tu.id,
						Name:  tu.name,
						Input: input,
					},
				})
			}

			a.history = append(a.history, anthropic.NewAssistantMessage(assistantBlocks...))

			// If no tool uses or stop reason is end_turn, we're done
			if len(toolUses) == 0 || stopReason == anthropic.StopReasonEndTurn {
				slog.Info("agent run completed", "agent", a.config.Name, "duration", time.Since(start), "iterations", iteration+1, "stopReason", stopReason)

				// Auto-compact if input tokens exceed threshold
				if inputTokens > int64(AutoCompactThreshold) {
					slog.Info("auto-compacting: input tokens exceed threshold",
						"agent", a.config.Name, "inputTokens", inputTokens, "threshold", AutoCompactThreshold)
					summary, compactErr := a.CompactHistory(ctx)
					if compactErr != nil {
						slog.Warn("auto-compact failed", "error", compactErr)
					} else {
						ch <- domain.StreamEvent{
							AgentName: a.config.Name,
							TokenUsage: &domain.TokenUsageEvent{
								InputTokens:  len(summary) / 4, // rough estimate after compaction
								OutputTokens: 0,
							},
						}
					}
				}

				ch <- domain.StreamEvent{AgentName: a.config.Name, Done: true}
				return
			}

			// Notify TUI of all tool calls up front
			for _, tu := range toolUses {
				ch <- domain.StreamEvent{
					AgentName: a.config.Name,
					ToolCall: &domain.ToolCallEvent{
						ToolName: tu.name,
						ToolID:   tu.id,
						Input:    tu.inputBuf.String(),
					},
				}
			}

			// Execute tools in parallel
			type toolResult struct {
				output  string
				isError bool
			}
			results := make([]toolResult, len(toolUses))

			var wg sync.WaitGroup
			for i, tu := range toolUses {
				wg.Add(1)
				go func(idx int, tu pendingToolUse) {
					defer wg.Done()
					slog.Debug("executing tool", "agent", a.config.Name, "tool", tu.name, "id", tu.id)
					result, execErr := ExecuteTool(ctx, a.WorkDir(), tu.name, json.RawMessage(tu.inputBuf.String()))
					if execErr != nil {
						results[idx] = toolResult{output: execErr.Error(), isError: true}
					} else {
						results[idx] = toolResult{output: result}
					}
				}(i, tu)
			}
			wg.Wait()

			// Emit results and build API response in original order
			var toolResultBlocks []anthropic.ContentBlockParamUnion
			for i, tu := range toolUses {
				r := results[i]
				ch <- domain.StreamEvent{
					AgentName: a.config.Name,
					ToolDone: &domain.ToolDoneEvent{
						ToolName: tu.name,
						ToolID:   tu.id,
						Output:   r.output,
						IsError:  r.isError,
					},
				}
				toolResultBlocks = append(toolResultBlocks, anthropic.ContentBlockParamUnion{
					OfToolResult: &anthropic.ToolResultBlockParam{
						ToolUseID: tu.id,
						Content: []anthropic.ToolResultBlockParamContentUnion{
							{OfText: &anthropic.TextBlockParam{Text: r.output}},
						},
					},
				})
			}

			a.history = append(a.history, anthropic.NewUserMessage(toolResultBlocks...))
			slog.Debug("tool results sent, continuing loop", "agent", a.config.Name, "toolCount", len(toolUses), "iteration", iteration)
		}

		slog.Warn("agent hit tool use iteration limit", "agent", a.config.Name)
		ch <- domain.StreamEvent{AgentName: a.config.Name, Done: true}
	}()

	return ch
}

// billingHeader is the Claude Code billing header that enables access to
// Opus/Sonnet models via OAuth Console login.
const billingHeader = "x-anthropic-billing-header: cc_version=2.1.87.7b6; cc_entrypoint=cli; cch=22c94;"

// claudeCodeBetas are the beta feature flags required for Claude Code billing.
const claudeCodeBetas = "claude-code-20250219,interleaved-thinking-2025-05-14,effort-2025-11-24"

// buildMessageParams constructs the API request params with tools.
func (a *Agent) buildMessageParams(model anthropic.Model) (anthropic.MessageNewParams, []option.RequestOption) {
	// Hot-reload CLAUDE.md if modified
	a.RefreshClaudeMD()

	system := []anthropic.TextBlockParam{
		{Text: a.systemPrompt},
	}

	defs := ToolDefinitions()
	toolNames := make([]string, len(defs))
	for i, d := range defs {
		toolNames[i] = d.Name
	}
	slog.Debug("building API request", "agent", a.config.Name, "toolCount", len(defs), "tools", toolNames)

	params := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: defaultMaxTokens,
		System:    system,
		Messages:  a.history,
		Tools:     ToAPITools(defs),
	}

	var reqOpts []option.RequestOption

	if a.isOAuth {
		params.System = append(
			[]anthropic.TextBlockParam{{Text: billingHeader}},
			params.System...,
		)
		params.Thinking = anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
		}
		params.OutputConfig = anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortMedium,
		}
		params.MaxTokens = oauthMaxTokens
		reqOpts = append(reqOpts, option.WithHeader("anthropic-beta", claudeCodeBetas))

		slog.Debug("injected Claude Code billing header for OAuth",
			"agent", a.config.Name, "model", string(model))
	}

	return params, reqOpts
}

// WorkDir returns the agent's working directory.
func (a *Agent) WorkDir() string {
	if a.dir != "" {
		return a.dir
	}
	return "."
}

// isRetryableError returns true for transient server errors that are safe to retry.
// This includes 500 (Internal Server Error), 502, 503, 529 (Overloaded), and
// streaming errors that contain "api_error" or "overloaded_error".
func isRetryableError(err error) bool {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case 500, 502, 503, 529:
			return true
		}
		return false
	}
	// Streaming errors aren't typed — check the error message
	msg := err.Error()
	return strings.Contains(msg, "api_error") ||
		strings.Contains(msg, "overloaded_error") ||
		strings.Contains(msg, "Internal server error")
}

// enrichAPIError adds actionable context to common API errors.
// Uses the SDK's typed error to inspect HTTP status codes directly
// rather than fragile string matching.
func enrichAPIError(err error, model string) error {
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return err
	}
	switch apiErr.StatusCode {
	case 400, 403:
		return fmt.Errorf("%w — your API key may not have access to %s. "+
			"Run `aql auth login --console` for full model access, "+
			"or /model to switch models", err, model)
	case 404:
		return fmt.Errorf("%w — model %q not found. Try /model to pick a valid model", err, model)
	case 500, 502, 503:
		return fmt.Errorf("%w — API server error. This is usually transient, try again", err)
	case 529:
		return fmt.Errorf("%w — API is overloaded, try again in a moment", err)
	default:
		return err
	}
}
