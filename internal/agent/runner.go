package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/amer/aql/internal/domain"
	"github.com/amer/aql/internal/models"
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

		a.history = append(a.history, domain.NewUserMessage(userMessage))

		model := models.ResolveModel(a.config.Model)
		slog.Debug("starting API call", "agent", a.config.Name, "model", model, "historyLength", len(a.history), "oauth", a.isOAuth)

		// Tool use loop: keep calling the API until we get end_turn
		for iteration := 0; iteration < maxToolIterations; iteration++ {
			params := a.buildChatParams(model)

			resp, err := a.streamWithRetry(ctx, ch, params)
			if err != nil {
				slog.Error("API error", "agent", a.config.Name, "error", err, "duration", time.Since(start), "iteration", iteration)
				ch <- domain.StreamEvent{
					AgentName: a.config.Name,
					Error:     enrichAPIError(err, model),
				}
				return
			}

			// Emit precise token counts from this API call
			if resp.InputTokens > 0 || resp.OutputTokens > 0 {
				ch <- domain.StreamEvent{
					AgentName: a.config.Name,
					TokenUsage: &domain.TokenUsageEvent{
						InputTokens:  resp.InputTokens,
						OutputTokens: resp.OutputTokens,
					},
				}
				slog.Debug("token usage", "agent", a.config.Name, "input", resp.InputTokens, "output", resp.OutputTokens)
			}

			// Build assistant message from text + tool_use blocks
			var assistantBlocks []domain.ContentBlock
			for _, text := range resp.TextParts {
				assistantBlocks = append(assistantBlocks, domain.TextBlock(text))
			}
			for _, tu := range resp.ToolUses {
				assistantBlocks = append(assistantBlocks, domain.ToolUseContentBlock(tu.ID, tu.Name, tu.Input))
			}
			a.history = append(a.history, domain.Message{
				Role:    domain.RoleAssistant,
				Content: assistantBlocks,
			})

			// If no tool uses or stop reason is end_turn, we're done
			if len(resp.ToolUses) == 0 || resp.StopReason == "end_turn" {
				slog.Info("agent run completed", "agent", a.config.Name, "duration", time.Since(start), "iterations", iteration+1, "stopReason", resp.StopReason)
				a.maybeAutoCompact(ctx, ch, int64(resp.InputTokens))
				ch <- domain.StreamEvent{AgentName: a.config.Name, Done: true}
				return
			}

			// Execute tools and feed results back
			toolResultBlocks := a.executeTools(ctx, ch, resp.ToolUses)
			a.history = append(a.history, domain.Message{
				Role:    domain.RoleUser,
				Content: toolResultBlocks,
			})
			slog.Debug("tool results sent, continuing loop", "agent", a.config.Name, "toolCount", len(resp.ToolUses), "iteration", iteration)
		}

		slog.Warn("agent hit tool use iteration limit", "agent", a.config.Name)
		ch <- domain.StreamEvent{AgentName: a.config.Name, Done: true}
	}()

	return ch
}

// streamWithRetry performs the streaming API call with retry logic for transient errors.
func (a *Agent) streamWithRetry(ctx context.Context, ch chan<- domain.StreamEvent, params domain.ChatParams) (*domain.ChatResponse, error) {
	onText := func(text string) {
		ch <- domain.StreamEvent{AgentName: a.config.Name, Text: text}
	}

	for attempt := 0; attempt <= maxStreamRetries; attempt++ {
		resp, err := a.chatClient.StreamMessage(ctx, params, onText)
		if err == nil {
			return resp, nil
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

// executeTools runs all pending tool calls in parallel, emits events to ch,
// and returns the tool result content blocks for the next API call.
func (a *Agent) executeTools(ctx context.Context, ch chan<- domain.StreamEvent, toolUses []domain.ChatToolUse) []domain.ContentBlock {
	// Notify TUI of all tool calls up front
	for _, tu := range toolUses {
		ch <- domain.StreamEvent{
			AgentName: a.config.Name,
			ToolCall: &domain.ToolCallEvent{
				ToolName: tu.Name,
				ToolID:   tu.ID,
				Input:    tu.Input,
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
		go func(idx int, tu domain.ChatToolUse) {
			defer wg.Done()
			slog.Debug("executing tool", "agent", a.config.Name, "tool", tu.Name, "id", tu.ID)
			result, execErr := a.toolExecutor(ctx, a.WorkDir(), tu.Name, json.RawMessage(tu.Input))
			if execErr != nil {
				results[idx] = toolResult{output: execErr.Error(), isError: true}
			} else {
				results[idx] = toolResult{output: result}
			}
		}(i, tu)
	}
	wg.Wait()

	// Emit results and build content blocks in original order
	var blocks []domain.ContentBlock
	for i, tu := range toolUses {
		r := results[i]
		ch <- domain.StreamEvent{
			AgentName: a.config.Name,
			ToolDone: &domain.ToolDoneEvent{
				ToolName: tu.Name,
				ToolID:   tu.ID,
				Output:   r.output,
				IsError:  r.isError,
			},
		}
		blocks = append(blocks, domain.ToolResultContentBlock(tu.ID, r.output, r.isError))
	}
	return blocks
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

// buildChatParams constructs the domain.ChatParams for an API call.
func (a *Agent) buildChatParams(model string) domain.ChatParams {
	// Hot-reload CLAUDE.md if modified
	a.RefreshClaudeMD()

	defs := tools.Definitions()
	toolDefs := make([]domain.ToolDef, len(defs))
	for i, d := range defs {
		toolDefs[i] = domain.ToolDef{
			Name:        d.Name,
			Description: d.Description,
			InputSchema: d.InputSchema,
		}
	}

	toolNames := make([]string, len(defs))
	for i, d := range defs {
		toolNames[i] = d.Name
	}
	slog.Debug("building API request", "agent", a.config.Name, "toolCount", len(defs), "tools", toolNames)

	maxTokens := defaultMaxTokens
	if a.isOAuth {
		maxTokens = oauthMaxTokens
	}

	return domain.ChatParams{
		Model:        model,
		System:       a.systemPrompt,
		Messages:     a.history,
		Tools:        toolDefs,
		MaxTokens:    maxTokens,
		OAuthBilling: a.isOAuth,
	}
}

// WorkDir returns the agent's working directory.
func (a *Agent) WorkDir() string {
	if a.dir != "" {
		return a.dir
	}
	return "."
}
