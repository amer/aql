// Package llm provides adapters that implement domain.ChatClient for
// specific LLM providers. Currently only Anthropic is supported.
package llm

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/amer/aql/internal/domain"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
)

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

	return &AnthropicClient{client: anthropic.NewClient(sdkOpts...)}
}

// StreamMessage implements domain.ChatClient. It sends the conversation to the
// Anthropic API via streaming, calling onText for each text delta, and returns
// the accumulated response.
func (c *AnthropicClient) StreamMessage(ctx context.Context, params domain.ChatParams, onText func(string)) (*domain.ChatResponse, error) {
	apiParams, reqOpts := buildAPIParams(params)
	stream := c.client.Messages.NewStreaming(ctx, apiParams, reqOpts...)
	return consumeStream(stream, onText)
}

// SendMessage implements domain.ChatClient. It sends a non-streaming request
// and returns the response.
func (c *AnthropicClient) SendMessage(ctx context.Context, params domain.ChatParams) (*domain.ChatResponse, error) {
	apiParams, reqOpts := buildAPIParams(params)
	resp, err := c.client.Messages.New(ctx, apiParams, reqOpts...)
	if err != nil {
		return nil, err
	}

	var textParts []string
	for _, block := range resp.Content {
		if block.Type == "text" {
			textParts = append(textParts, block.Text)
		}
	}

	return &domain.ChatResponse{
		TextParts:    textParts,
		StopReason:   string(resp.StopReason),
		InputTokens:  int(resp.Usage.InputTokens),
		OutputTokens: int(resp.Usage.OutputTokens),
	}, nil
}

// buildAPIParams converts domain.ChatParams into SDK-specific request params.
func buildAPIParams(params domain.ChatParams) (anthropic.MessageNewParams, []option.RequestOption) {
	system := []anthropic.TextBlockParam{{Text: params.System}}

	apiParams := anthropic.MessageNewParams{
		Model:     anthropic.Model(params.Model),
		MaxTokens: int64(params.MaxTokens),
		System:    system,
		Messages:  toAPIMessages(params.Messages),
		Tools:     toAPITools(params.Tools),
	}

	var reqOpts []option.RequestOption

	if params.OAuthBilling {
		apiParams.System = append(
			[]anthropic.TextBlockParam{{Text: domain.BillingHeader}},
			apiParams.System...,
		)
		apiParams.Thinking = anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
		}
		apiParams.OutputConfig = anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortMedium,
		}
		reqOpts = append(reqOpts, option.WithHeader("anthropic-beta", domain.ClaudeCodeBetas))
	}

	return apiParams, reqOpts
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
		case b.ToolUse != nil:
			var input []byte
			if b.ToolUse.Input != "" {
				input = []byte(b.ToolUse.Input)
			} else {
				input = []byte("{}")
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

// consumeStream reads a streaming response, calling onText for text deltas,
// and returns the accumulated ChatResponse.
func consumeStream(stream *ssestream.Stream[anthropic.MessageStreamEventUnion], onText func(string)) (*domain.ChatResponse, error) {
	type pendingToolUse struct {
		id       string
		name     string
		inputBuf strings.Builder
	}

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

	// Build text parts in index order
	textIndices := make([]int64, 0, len(textBlocks))
	for idx := range textBlocks {
		textIndices = append(textIndices, idx)
	}
	slices.Sort(textIndices)

	var textParts []string
	for _, idx := range textIndices {
		text := textBlocks[idx].String()
		if text != "" {
			textParts = append(textParts, text)
		}
	}

	// Convert tool uses to domain type
	chatToolUses := make([]domain.ChatToolUse, len(toolUses))
	for i, tu := range toolUses {
		chatToolUses[i] = domain.ChatToolUse{
			ID:    tu.id,
			Name:  tu.name,
			Input: tu.inputBuf.String(),
		}
	}

	slog.Debug("stream consumed", "textParts", len(textParts), "toolUses", len(chatToolUses), "stopReason", stopReason)

	return &domain.ChatResponse{
		TextParts:    textParts,
		ToolUses:     chatToolUses,
		StopReason:   string(stopReason),
		InputTokens:  int(inputTokens),
		OutputTokens: int(outputTokens),
	}, nil
}
