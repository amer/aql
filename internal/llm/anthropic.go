// Package llm provides adapters that implement domain.ChatClient for
// specific LLM providers. Currently only Anthropic is supported.
package llm

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - AnthropicClient — implements domain.ChatClient
//   - StreamMessage/SendMessage methods
//   - APIError wrapper (satisfies statusCoder)
//   - buildAPIParams — converts domain types to SDK types
//   - applyOAuthConfig — billing headers + thinking
//   - Type conversion functions (toAPIMessages, toAPIContentBlocks,
//     toAPITools)
//   - consumeStream — SSE stream processing (text, thinking, tool_use,
//     token usage from message_start + message_delta)
//   - pendingToolUse / pendingThinking accumulators
//
// MUST NOT GO HERE:
//   - Domain type definitions (domain/types.go)
//   - Tool implementations (tools/)
//   - Agent logic, TUI imports
//   - This is a pure adapter — translates between domain types
//     and Anthropic SDK types.
//
// Q: Should I add a new LLM provider?
// A: Create a new file (e.g., openai.go) implementing
//    domain.ChatClient. Don't modify this file.
//
// Q: Should I add billing/thinking config?
// A: Update applyOAuthConfig() here.
//
// Q: How do streaming events work?
// A: consumeStream() reads the SSE stream, accumulates text/thinking/tool
//    blocks, captures input_tokens from message_start, calls onText for
//    text deltas.
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/amer/aql/internal/domain"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
)

// APIError wraps an SDK error and exposes the status code via a method,
// satisfying the statusCoder interface used by agent error handling.
type APIError struct {
	err        *anthropic.Error
	statusCode int
}

func (e *APIError) Error() string   { return e.err.Error() }
func (e *APIError) Unwrap() error   { return e.err }
func (e *APIError) StatusCode() int { return e.statusCode }

// wrapError converts *anthropic.Error into *APIError so downstream code
// can use the statusCoder interface instead of the concrete SDK type.
func wrapError(err error) error {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		return &APIError{err: apiErr, statusCode: apiErr.StatusCode}
	}
	return err
}

// AnthropicClient implements domain.ChatClient using the Anthropic SDK.
type AnthropicClient struct {
	client anthropic.Client
}

// ClientOption configures the Anthropic client construction.
type ClientOption func(*clientOptions)

type clientOptions struct {
	apiKey      string
	bearerToken string
	baseURL     string
	httpClient  *http.Client
}

// WithAPIKey sets the API key for authentication.
func WithAPIKey(key string) ClientOption {
	return func(o *clientOptions) { o.apiKey = key }
}

// WithBearerToken sets a Bearer token for authentication.
func WithBearerToken(token string) ClientOption {
	return func(o *clientOptions) { o.bearerToken = token }
}

// WithBaseURL sets a custom API base URL (useful for testing).
func WithBaseURL(url string) ClientOption {
	return func(o *clientOptions) { o.baseURL = url }
}

// WithHTTPClient sets a custom HTTP client for all API requests.
// Use this to inject a shared transport for recording, stubbing, or proxying.
func WithHTTPClient(c *http.Client) ClientOption {
	return func(o *clientOptions) { o.httpClient = c }
}

// NewAnthropicClient creates a new Anthropic adapter.
func NewAnthropicClient(opts ...ClientOption) *AnthropicClient {
	var o clientOptions
	for _, opt := range opts {
		opt(&o)
	}

	var sdkOpts []option.RequestOption
	if o.bearerToken != "" {
		sdkOpts = append(sdkOpts, option.WithAuthToken(o.bearerToken))
	} else if o.apiKey != "" {
		sdkOpts = append(sdkOpts, option.WithAPIKey(o.apiKey))
	}
	if o.baseURL != "" {
		sdkOpts = append(sdkOpts, option.WithBaseURL(o.baseURL))
	}
	if o.httpClient != nil {
		sdkOpts = append(sdkOpts, option.WithHTTPClient(o.httpClient))
	}

	return &AnthropicClient{client: anthropic.NewClient(sdkOpts...)}
}

// StreamMessage implements domain.ChatClient. It sends the conversation to the
// Anthropic API via streaming, calling onText for each text delta, and returns
// the accumulated response.
func (c *AnthropicClient) StreamMessage(ctx context.Context, params domain.ChatParams, onText func(string)) (*domain.ChatResponse, error) {
	apiParams, reqOpts := buildAPIParams(params)
	stream := c.client.Messages.NewStreaming(ctx, apiParams, reqOpts...)
	resp, err := consumeStream(stream, onText)
	if err != nil {
		return nil, wrapError(err)
	}
	return resp, nil
}

// SendMessage implements domain.ChatClient. It sends a non-streaming request
// and returns the response.
func (c *AnthropicClient) SendMessage(ctx context.Context, params domain.ChatParams) (*domain.ChatResponse, error) {
	apiParams, reqOpts := buildAPIParams(params)
	resp, err := c.client.Messages.New(ctx, apiParams, reqOpts...)
	if err != nil {
		return nil, wrapError(err)
	}

	var textParts []string
	var thinking []domain.ChatThinking
	var toolUses []domain.ChatToolUse
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "thinking":
			thinking = append(thinking, domain.ChatThinking{Text: block.Thinking, Signature: block.Signature})
		case "tool_use":
			toolUses = append(toolUses, domain.ChatToolUse{ID: block.ID, Name: block.Name, Input: string(block.Input)})
		}
	}

	return &domain.ChatResponse{
		TextParts:    textParts,
		Thinking:     thinking,
		ToolUses:     toolUses,
		StopReason:   string(resp.StopReason),
		InputTokens:  int(resp.Usage.InputTokens),
		OutputTokens: int(resp.Usage.OutputTokens),
	}, nil
}

// buildAPIParams converts domain.ChatParams into SDK-specific request params.
func buildAPIParams(params domain.ChatParams) (anthropic.MessageNewParams, []option.RequestOption) {
	apiParams := anthropic.MessageNewParams{
		Model:     anthropic.Model(params.Model),
		MaxTokens: int64(params.MaxTokens),
		System:    []anthropic.TextBlockParam{{Text: params.System}},
		Messages:  toAPIMessages(params.Messages),
		Tools:     toAPITools(params.Tools),
	}

	var reqOpts []option.RequestOption
	if params.OAuthBilling {
		reqOpts = applyOAuthConfig(&apiParams)
	}

	return apiParams, reqOpts
}

// applyOAuthConfig injects billing headers, adaptive thinking, and effort
// config required for OAuth-authenticated requests.
func applyOAuthConfig(p *anthropic.MessageNewParams) []option.RequestOption {
	p.System = append(
		[]anthropic.TextBlockParam{{Text: domain.BillingHeader}},
		p.System...,
	)
	p.Thinking = anthropic.ThinkingConfigParamUnion{
		OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
	}
	p.OutputConfig = anthropic.OutputConfigParam{
		Effort: anthropic.OutputConfigEffortMedium,
	}
	return []option.RequestOption{option.WithHeader("anthropic-beta", domain.ClaudeCodeBetas)}
}

// toAPIMessages converts domain messages to SDK message params.
func toAPIMessages(msgs []domain.Message) []anthropic.MessageParam {
	result := make([]anthropic.MessageParam, len(msgs))
	for i, msg := range msgs {
		result[i] = anthropic.MessageParam{
			Role:    anthropic.MessageParamRole(msg.Role),
			Content: toAPIContentBlocks(msg.Content),
		}
	}
	return result
}

// toAPIContentBlocks converts domain content blocks to SDK content block params.
func toAPIContentBlocks(blocks []domain.ContentBlock) []anthropic.ContentBlockParamUnion {
	result := make([]anthropic.ContentBlockParamUnion, len(blocks))
	for i, b := range blocks {
		switch {
		case b.Thinking != nil:
			result[i] = anthropic.NewThinkingBlock(b.Thinking.Signature, b.Thinking.Text)
		case b.ToolUse != nil:
			var input json.RawMessage
			if b.ToolUse.Input != "" {
				input = json.RawMessage(b.ToolUse.Input)
			} else {
				input = json.RawMessage("{}")
			}
			result[i] = anthropic.ContentBlockParamUnion{
				OfToolUse: &anthropic.ToolUseBlockParam{
					ID:    b.ToolUse.ID,
					Name:  b.ToolUse.Name,
					Input: input,
				},
			}
		case b.ToolResult != nil:
			result[i] = anthropic.ContentBlockParamUnion{
				OfToolResult: &anthropic.ToolResultBlockParam{
					ToolUseID: b.ToolResult.ToolUseID,
					Content: []anthropic.ToolResultBlockParamContentUnion{
						{OfText: &anthropic.TextBlockParam{Text: b.ToolResult.Content}},
					},
				},
			}
		default:
			result[i] = anthropic.NewTextBlock(b.Text)
		}
	}
	return result
}

// toAPITools converts domain tool definitions to SDK tool params.
func toAPITools(defs []domain.ToolDef) []anthropic.ToolUnionParam {
	if len(defs) == 0 {
		return nil
	}
	tools := make([]anthropic.ToolUnionParam, len(defs))
	for i, d := range defs {
		tools[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        d.Name,
				Description: anthropic.String(d.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: d.InputSchema["properties"],
					Required:   toStringSlice(d.InputSchema["required"]),
				},
			},
		}
	}
	return tools
}

func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		result := make([]string, len(s))
		for i, item := range s {
			result[i] = fmt.Sprint(item)
		}
		return result
	}
	return nil
}

// pendingToolUse accumulates input JSON deltas for a single tool_use block.
type pendingToolUse struct {
	id       string
	name     string
	inputBuf strings.Builder
}

// pendingThinking accumulates thinking-text deltas and the signature for a
// single thinking block.
type pendingThinking struct {
	textBuf   strings.Builder
	signature string
}

// consumeStream reads a streaming response, calling onText for text deltas,
// and returns the accumulated ChatResponse.
func consumeStream(stream *ssestream.Stream[anthropic.MessageStreamEventUnion], onText func(string)) (*domain.ChatResponse, error) {

	var toolUses []*pendingToolUse
	var thinking []*pendingThinking
	var stopReason anthropic.StopReason
	var inputTokens, outputTokens int64
	activeBlocks := map[int64]*pendingToolUse{}
	textBlocks := map[int64]*strings.Builder{}
	thinkingBlocks := map[int64]*pendingThinking{}

	for stream.Next() {
		evt := stream.Current()

		switch v := evt.AsAny().(type) {
		case anthropic.MessageStartEvent:
			// message_start is the documented carrier of input_tokens; without
			// this the response reports 0 and auto-compact never fires.
			if v.Message.Usage.InputTokens > 0 {
				inputTokens = v.Message.Usage.InputTokens
			}
			if v.Message.Usage.OutputTokens > 0 {
				outputTokens = v.Message.Usage.OutputTokens
			}

		case anthropic.ContentBlockStartEvent:
			cb := v.ContentBlock
			switch cb.Type {
			case "text":
				textBlocks[v.Index] = &strings.Builder{}
			case "thinking":
				th := &pendingThinking{}
				thinkingBlocks[v.Index] = th
				thinking = append(thinking, th)
			case "tool_use":
				tu := &pendingToolUse{id: cb.ID, name: cb.Name}
				activeBlocks[v.Index] = tu
				toolUses = append(toolUses, tu)
			}

		case anthropic.ContentBlockDeltaEvent:
			switch d := v.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				if d.Text != "" {
					if onText != nil {
						onText(d.Text)
					}
					if sb, ok := textBlocks[v.Index]; ok {
						sb.WriteString(d.Text)
					}
				}
			case anthropic.InputJSONDelta:
				if tu, ok := activeBlocks[v.Index]; ok {
					tu.inputBuf.WriteString(d.PartialJSON)
				}
			case anthropic.ThinkingDelta:
				if th, ok := thinkingBlocks[v.Index]; ok {
					th.textBuf.WriteString(d.Thinking)
				}
			case anthropic.SignatureDelta:
				if th, ok := thinkingBlocks[v.Index]; ok {
					th.signature = d.Signature
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

	return buildStreamResponse(textBlocks, thinking, toolUses, stopReason, inputTokens, outputTokens), nil
}

// buildStreamResponse assembles the final ChatResponse from accumulated stream state.
func buildStreamResponse(textBlocks map[int64]*strings.Builder, thinking []*pendingThinking, toolUses []*pendingToolUse, stopReason anthropic.StopReason, inputTokens, outputTokens int64) *domain.ChatResponse {
	textParts := collectTextParts(textBlocks)
	chatThinking := make([]domain.ChatThinking, len(thinking))
	for i, th := range thinking {
		chatThinking[i] = domain.ChatThinking{
			Text:      th.textBuf.String(),
			Signature: th.signature,
		}
	}
	chatToolUses := make([]domain.ChatToolUse, len(toolUses))
	for i, tu := range toolUses {
		chatToolUses[i] = domain.ChatToolUse{
			ID:    tu.id,
			Name:  tu.name,
			Input: tu.inputBuf.String(),
		}
	}

	slog.Debug("stream consumed", "textParts", len(textParts), "thinking", len(chatThinking), "toolUses", len(chatToolUses), "stopReason", stopReason)

	return &domain.ChatResponse{
		TextParts:    textParts,
		Thinking:     chatThinking,
		ToolUses:     chatToolUses,
		StopReason:   string(stopReason),
		InputTokens:  int(inputTokens),
		OutputTokens: int(outputTokens),
	}
}

// collectTextParts extracts non-empty text strings from text blocks in index order.
func collectTextParts(textBlocks map[int64]*strings.Builder) []string {
	indices := make([]int64, 0, len(textBlocks))
	for idx := range textBlocks {
		indices = append(indices, idx)
	}
	slices.Sort(indices)

	var parts []string
	for _, idx := range indices {
		if text := textBlocks[idx].String(); text != "" {
			parts = append(parts, text)
		}
	}
	return parts
}
