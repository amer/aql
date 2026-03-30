package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
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
	ToolCall  *ToolCallEvent // non-nil when agent invokes a tool
	ToolDone  *ToolDoneEvent // non-nil when a tool finishes
}

// ToolCallEvent is emitted when the agent starts a tool call.
type ToolCallEvent struct {
	ToolName string
	ToolID   string
	Input    string
}

// ToolDoneEvent is emitted when a tool call completes.
type ToolDoneEvent struct {
	ToolName string
	ToolID   string
	Output   string
	IsError  bool
}

// Run sends a user message to Claude and streams responses back on the
// returned channel. Implements a tool use loop: if Claude responds with
// tool_use blocks, the tools are executed and results sent back until
// Claude produces a final text response.
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
		slog.Debug("starting API call", "agent", a.config.Name, "model", string(model), "historyLength", len(a.history), "oauth", a.isOAuth)

		// Tool use loop: keep calling the API until we get end_turn
		for iteration := 0; iteration < 25; iteration++ {
			params, reqOpts := a.buildMessageParams(model)

			stream := a.client.Messages.NewStreaming(ctx, params, reqOpts...)

			// Accumulate content blocks from the stream
			var assistantBlocks []anthropic.ContentBlockParamUnion
			type pendingToolUse struct {
				id       string
				name     string
				inputBuf strings.Builder
			}
			var toolUses []pendingToolUse
			var stopReason anthropic.StopReason

			// Track active block by index
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
							ch <- StreamEvent{AgentName: a.config.Name, Text: d.Text}
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
				}
			}

			if err := stream.Err(); err != nil {
				slog.Error("API error", "agent", a.config.Name, "error", err, "duration", time.Since(start), "iteration", iteration)
				ch <- StreamEvent{
					AgentName: a.config.Name,
					Error:     enrichAPIError(err, string(model)),
				}
				return
			}
			stream.Close()

			// Build assistant message from accumulated blocks
			for _, sb := range textBlocks {
				text := sb.String()
				if text != "" {
					assistantBlocks = append(assistantBlocks, anthropic.NewTextBlock(text))
				}
			}
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
				ch <- StreamEvent{AgentName: a.config.Name, Done: true}
				return
			}

			// Notify TUI of all tool calls up front
			for _, tu := range toolUses {
				ch <- StreamEvent{
					AgentName: a.config.Name,
					ToolCall: &ToolCallEvent{
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
				ch <- StreamEvent{
					AgentName: a.config.Name,
					ToolDone: &ToolDoneEvent{
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
		ch <- StreamEvent{AgentName: a.config.Name, Done: true}
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
		MaxTokens: 4096,
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
		params.MaxTokens = 16384
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
