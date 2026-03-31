package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/amer/aql/internal/domain"
	"github.com/amer/aql/internal/models"
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

// pendingToolUse accumulates a tool_use block as it streams in.
type pendingToolUse struct {
	id       string
	name     string
	inputBuf strings.Builder
}

// streamResult holds the accumulated output of a single API streaming call.
type streamResult struct {
	textBlocks   []anthropic.ContentBlockParamUnion
	toolUses     []pendingToolUse
	stopReason   anthropic.StopReason
	inputTokens  int64
	outputTokens int64
}

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

		model := models.ResolveModel(a.config.Model)
		slog.Debug("starting API call", "agent", a.config.Name, "model", string(model), "historyLength", len(a.history), "oauth", a.isOAuth)

		// Tool use loop: keep calling the API until we get end_turn
		for iteration := 0; iteration < maxToolIterations; iteration++ {
			params, reqOpts := a.buildMessageParams(model)

			result, err := a.streamWithRetry(ctx, ch, params, reqOpts)
			if err != nil {
				slog.Error("API error", "agent", a.config.Name, "error", err, "duration", time.Since(start), "iteration", iteration)
				ch <- domain.StreamEvent{
					AgentName: a.config.Name,
					Error:     enrichAPIError(err, string(model)),
				}
				return
			}

			// Emit precise token counts from this API call
			if result.inputTokens > 0 || result.outputTokens > 0 {
				ch <- domain.StreamEvent{
					AgentName: a.config.Name,
					TokenUsage: &domain.TokenUsageEvent{
						InputTokens:  int(result.inputTokens),
						OutputTokens: int(result.outputTokens),
					},
				}
				slog.Debug("token usage", "agent", a.config.Name, "input", result.inputTokens, "output", result.outputTokens)
			}

			// Build assistant message from text + tool_use blocks
			assistantBlocks := result.textBlocks
			for _, tu := range result.toolUses {
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
			if len(result.toolUses) == 0 || result.stopReason == anthropic.StopReasonEndTurn {
				slog.Info("agent run completed", "agent", a.config.Name, "duration", time.Since(start), "iterations", iteration+1, "stopReason", result.stopReason)
				a.maybeAutoCompact(ctx, ch, result.inputTokens)
				ch <- domain.StreamEvent{AgentName: a.config.Name, Done: true}
				return
			}

			// Execute tools and feed results back
			toolResultBlocks := a.executeTools(ctx, ch, result.toolUses)
			a.history = append(a.history, anthropic.NewUserMessage(toolResultBlocks...))
			slog.Debug("tool results sent, continuing loop", "agent", a.config.Name, "toolCount", len(result.toolUses), "iteration", iteration)
		}

		slog.Warn("agent hit tool use iteration limit", "agent", a.config.Name)
		ch <- domain.StreamEvent{AgentName: a.config.Name, Done: true}
	}()

	return ch
}

// streamWithRetry performs the streaming API call with retry logic for transient errors.
// Returns the accumulated stream result or an error.
func (a *Agent) streamWithRetry(ctx context.Context, ch chan<- domain.StreamEvent, params anthropic.MessageNewParams, reqOpts []option.RequestOption) (*streamResult, error) {
	for attempt := 0; attempt <= maxStreamRetries; attempt++ {
		result, err := a.consumeStream(ctx, ch, params, reqOpts)
		if err == nil {
			return result, nil
		}

		if !isRetryableError(err) || attempt == maxStreamRetries {
			return nil, err
		}

		delay := streamRetryBaseDelay * time.Duration(1<<attempt)
		slog.Warn("transient API error, retrying",
			"agent", a.config.Name, "error", err, "attempt", attempt+1, "maxRetries", maxStreamRetries, "delay", delay)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("exhausted retries")
}

// consumeStream reads a single streaming response, emitting text deltas to ch
// and accumulating the full response into a streamResult.
func (a *Agent) consumeStream(ctx context.Context, ch chan<- domain.StreamEvent, params anthropic.MessageNewParams, reqOpts []option.RequestOption) (*streamResult, error) {
	stream := a.client.Messages.NewStreaming(ctx, params, reqOpts...)

	var toolUses []pendingToolUse
	var stopReason anthropic.StopReason
	var inputTokens, outputTokens int64
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

	if streamErr != nil {
		return nil, streamErr
	}

	// Build text blocks in index order
	textIndices := make([]int64, 0, len(textBlocks))
	for idx := range textBlocks {
		textIndices = append(textIndices, idx)
	}
	sort.Slice(textIndices, func(i, j int) bool { return textIndices[i] < textIndices[j] })

	var blocks []anthropic.ContentBlockParamUnion
	for _, idx := range textIndices {
		text := textBlocks[idx].String()
		if text != "" {
			blocks = append(blocks, anthropic.NewTextBlock(text))
		}
	}

	return &streamResult{
		textBlocks:   blocks,
		toolUses:     toolUses,
		stopReason:   stopReason,
		inputTokens:  inputTokens,
		outputTokens: outputTokens,
	}, nil
}

// executeTools runs all pending tool calls in parallel, emits events to ch,
// and returns the tool result blocks for the next API call.
func (a *Agent) executeTools(ctx context.Context, ch chan<- domain.StreamEvent, toolUses []pendingToolUse) []anthropic.ContentBlockParamUnion {
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
			result, execErr := a.toolExecutor(ctx, a.WorkDir(), tu.name, json.RawMessage(tu.inputBuf.String()))
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
	return toolResultBlocks
}

// maybeAutoCompact triggers compaction if input tokens exceed the threshold.
func (a *Agent) maybeAutoCompact(ctx context.Context, ch chan<- domain.StreamEvent, inputTokens int64) {
	if inputTokens <= int64(AutoCompactThreshold) {
		return
	}
	slog.Info("auto-compacting: input tokens exceed threshold",
		"agent", a.config.Name, "inputTokens", inputTokens, "threshold", AutoCompactThreshold)
	summary, compactErr := a.CompactHistory(ctx)
	if compactErr != nil {
		slog.Warn("auto-compact failed", "error", compactErr)
		return
	}
	ch <- domain.StreamEvent{
		AgentName: a.config.Name,
		TokenUsage: &domain.TokenUsageEvent{
			InputTokens:  len(summary) / 4, // rough estimate after compaction
			OutputTokens: 0,
		},
	}
}

// buildMessageParams constructs the API request params with tools.
func (a *Agent) buildMessageParams(model anthropic.Model) (anthropic.MessageNewParams, []option.RequestOption) {
	// Hot-reload CLAUDE.md if modified
	a.RefreshClaudeMD()

	system := []anthropic.TextBlockParam{
		{Text: a.systemPrompt},
	}

	defs := tools.Definitions()
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
		Tools:     tools.ToAPITools(defs),
	}

	var reqOpts []option.RequestOption

	if a.isOAuth {
		params.System = append(
			[]anthropic.TextBlockParam{{Text: domain.BillingHeader}},
			params.System...,
		)
		params.Thinking = anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
		}
		params.OutputConfig = anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortMedium,
		}
		params.MaxTokens = oauthMaxTokens
		reqOpts = append(reqOpts, option.WithHeader("anthropic-beta", domain.ClaudeCodeBetas))

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
