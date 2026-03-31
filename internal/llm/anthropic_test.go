package llm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amer/aql/internal/domain"
	"github.com/amer/aql/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- StreamMessage ---

func TestStreamMessage_TextDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseTextResponse("hello", " world"))
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithAPIKey("test-key"))

	var deltas []string
	resp, err := client.StreamMessage(
		context.Background(),
		domain.ChatParams{
			Model:     "claude-sonnet-4-6",
			System:    "Be helpful.",
			Messages:  []domain.Message{domain.NewUserMessage("hi")},
			MaxTokens: 1024,
		},
		func(text string) { deltas = append(deltas, text) },
	)

	require.NoError(t, err)
	assert.Equal(t, []string{"hello", " world"}, deltas, "onText should receive each delta")
	assert.Equal(t, []string{"hello world"}, resp.TextParts)
	assert.Equal(t, "end_turn", resp.StopReason)
}

func TestStreamMessage_ToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseToolUseResponse("tu_1", "read_file", `{"path":"main.go"}`))
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithAPIKey("test-key"))

	resp, err := client.StreamMessage(
		context.Background(),
		domain.ChatParams{
			Model:     "claude-sonnet-4-6",
			System:    "Use tools.",
			Messages:  []domain.Message{domain.NewUserMessage("read main.go")},
			MaxTokens: 1024,
		},
		nil,
	)

	require.NoError(t, err)
	require.Len(t, resp.ToolUses, 1)
	assert.Equal(t, "tu_1", resp.ToolUses[0].ID)
	assert.Equal(t, "read_file", resp.ToolUses[0].Name)
	assert.Equal(t, `{"path":"main.go"}`, resp.ToolUses[0].Input)
	assert.Equal(t, "tool_use", resp.StopReason)
}

func TestStreamMessage_TokenCounts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseTextResponseWithTokens("ok", 150, 10))
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithAPIKey("test-key"))

	resp, err := client.StreamMessage(
		context.Background(),
		domain.ChatParams{
			Model:     "claude-sonnet-4-6",
			System:    "test",
			Messages:  []domain.Message{domain.NewUserMessage("hi")},
			MaxTokens: 1024,
		},
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, 150, resp.InputTokens)
	assert.Equal(t, 10, resp.OutputTokens)
}

func TestStreamMessage_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"type":"invalid_request_error","message":"bad request"}}`)
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithAPIKey("test-key"))

	_, err := client.StreamMessage(
		context.Background(),
		domain.ChatParams{
			Model:     "claude-sonnet-4-6",
			System:    "test",
			Messages:  []domain.Message{domain.NewUserMessage("hi")},
			MaxTokens: 1024,
		},
		nil,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

// --- SendMessage ---

func TestSendMessage_ReturnsTextParts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id": "msg_1",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "hello world"}],
			"model": "claude-sonnet-4-6",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 50, "output_tokens": 5}
		}`)
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithAPIKey("test-key"))

	resp, err := client.SendMessage(
		context.Background(),
		domain.ChatParams{
			Model:     "claude-sonnet-4-6",
			System:    "test",
			Messages:  []domain.Message{domain.NewUserMessage("hi")},
			MaxTokens: 1024,
		},
	)

	require.NoError(t, err)
	assert.Equal(t, []string{"hello world"}, resp.TextParts)
	assert.Equal(t, "end_turn", resp.StopReason)
	assert.Equal(t, 50, resp.InputTokens)
	assert.Equal(t, 5, resp.OutputTokens)
}

func TestSendMessage_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":{"type":"server_error","message":"internal error"}}`)
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithAPIKey("test-key"))

	_, err := client.SendMessage(
		context.Background(),
		domain.ChatParams{
			Model:     "claude-sonnet-4-6",
			System:    "test",
			Messages:  []domain.Message{domain.NewUserMessage("hi")},
			MaxTokens: 1024,
		},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// --- buildAPIParams (tested via captured request body) ---

func TestBuildAPIParams_SetsModelAndSystem(t *testing.T) {
	var captured map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseTextResponse("ok"))
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithAPIKey("test-key"))

	client.StreamMessage(context.Background(), domain.ChatParams{
		Model:     "claude-opus-4-6",
		System:    "You are helpful.",
		Messages:  []domain.Message{domain.NewUserMessage("hi")},
		MaxTokens: 2048,
	}, nil)

	assert.Equal(t, "claude-opus-4-6", captured["model"])
	assert.Equal(t, float64(2048), captured["max_tokens"])

	system, ok := captured["system"].([]any)
	require.True(t, ok)
	require.Len(t, system, 1)
	block := system[0].(map[string]any)
	assert.Equal(t, "You are helpful.", block["text"])
}

func TestBuildAPIParams_OAuthBillingInjectsHeaders(t *testing.T) {
	var captured map[string]any
	var betaHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		betaHeader = r.Header.Get("Anthropic-Beta")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseTextResponse("ok"))
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithAPIKey("test-key"))

	client.StreamMessage(context.Background(), domain.ChatParams{
		Model:        "claude-opus-4-6",
		System:       "Be helpful.",
		Messages:     []domain.Message{domain.NewUserMessage("hi")},
		MaxTokens:    16384,
		OAuthBilling: true,
	}, nil)

	// Billing header prepended to system
	system := captured["system"].([]any)
	require.True(t, len(system) >= 2, "should have billing + actual system prompt")
	billingText := system[0].(map[string]any)["text"].(string)
	assert.Contains(t, billingText, "x-anthropic-billing-header:")

	// Adaptive thinking
	thinking := captured["thinking"].(map[string]any)
	assert.Equal(t, "adaptive", thinking["type"])

	// Output config effort
	outputConfig := captured["output_config"].(map[string]any)
	assert.Equal(t, "medium", outputConfig["effort"])

	// Beta headers
	assert.Contains(t, betaHeader, "claude-code-20250219")
	assert.Contains(t, betaHeader, "interleaved-thinking-2025-05-14")
}

func TestBuildAPIParams_NoOAuth_NoBillingHeaders(t *testing.T) {
	var captured map[string]any
	var betaHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		betaHeader = r.Header.Get("Anthropic-Beta")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseTextResponse("ok"))
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithAPIKey("test-key"))

	client.StreamMessage(context.Background(), domain.ChatParams{
		Model:        "claude-sonnet-4-6",
		System:       "Be helpful.",
		Messages:     []domain.Message{domain.NewUserMessage("hi")},
		MaxTokens:    4096,
		OAuthBilling: false,
	}, nil)

	// System should have exactly 1 block (no billing header)
	system := captured["system"].([]any)
	assert.Len(t, system, 1)

	// No thinking or output_config
	assert.Nil(t, captured["thinking"])
	assert.Nil(t, captured["output_config"])

	// No beta headers
	assert.Empty(t, betaHeader)
}

// --- Message conversion (tested via captured request body) ---

func TestMessageConversion_AllContentBlockTypes(t *testing.T) {
	var captured map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseTextResponse("ok"))
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithAPIKey("test-key"))

	client.StreamMessage(context.Background(), domain.ChatParams{
		Model:  "claude-sonnet-4-6",
		System: "test",
		Messages: []domain.Message{
			domain.NewUserMessage("read main.go"),
			{
				Role: domain.RoleAssistant,
				Content: []domain.ContentBlock{
					domain.TextBlock("I'll read it."),
					domain.ToolUseContentBlock("tu_1", "read_file", `{"path":"main.go"}`),
				},
			},
			{
				Role: domain.RoleUser,
				Content: []domain.ContentBlock{
					domain.ToolResultContentBlock("tu_1", "package main", false),
				},
			},
		},
		MaxTokens: 1024,
	}, nil)

	msgs := captured["messages"].([]any)
	require.Len(t, msgs, 3)

	// Message 1: user text
	msg1 := msgs[0].(map[string]any)
	assert.Equal(t, "user", msg1["role"])
	blocks1 := msg1["content"].([]any)
	require.Len(t, blocks1, 1)
	assert.Equal(t, "text", blocks1[0].(map[string]any)["type"])
	assert.Equal(t, "read main.go", blocks1[0].(map[string]any)["text"])

	// Message 2: assistant text + tool_use
	msg2 := msgs[1].(map[string]any)
	assert.Equal(t, "assistant", msg2["role"])
	blocks2 := msg2["content"].([]any)
	require.Len(t, blocks2, 2)
	assert.Equal(t, "text", blocks2[0].(map[string]any)["type"])
	assert.Equal(t, "tool_use", blocks2[1].(map[string]any)["type"])
	assert.Equal(t, "tu_1", blocks2[1].(map[string]any)["id"])
	assert.Equal(t, "read_file", blocks2[1].(map[string]any)["name"])

	// Message 3: user tool_result
	msg3 := msgs[2].(map[string]any)
	assert.Equal(t, "user", msg3["role"])
	blocks3 := msg3["content"].([]any)
	require.Len(t, blocks3, 1)
	assert.Equal(t, "tool_result", blocks3[0].(map[string]any)["type"])
	assert.Equal(t, "tu_1", blocks3[0].(map[string]any)["tool_use_id"])
}

// --- tool_use input serialization ---

func TestToolUseInput_SerializedAsDictionary(t *testing.T) {
	var captured map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseTextResponse("ok"))
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithAPIKey("test-key"))

	client.StreamMessage(context.Background(), domain.ChatParams{
		Model:  "claude-sonnet-4-6",
		System: "test",
		Messages: []domain.Message{
			domain.NewUserMessage("read main.go"),
			{
				Role: domain.RoleAssistant,
				Content: []domain.ContentBlock{
					domain.ToolUseContentBlock("tu_1", "read_file", `{"path":"main.go"}`),
				},
			},
			{
				Role: domain.RoleUser,
				Content: []domain.ContentBlock{
					domain.ToolResultContentBlock("tu_1", "package main", false),
				},
			},
		},
		MaxTokens: 1024,
	}, nil)

	msgs := captured["messages"].([]any)
	assistantMsg := msgs[1].(map[string]any)
	blocks := assistantMsg["content"].([]any)
	toolUse := blocks[0].(map[string]any)

	// The input field MUST be a JSON object (map), not a string or base64-encoded bytes.
	input, ok := toolUse["input"].(map[string]any)
	require.True(t, ok, "tool_use input must be a dictionary, got %T: %v", toolUse["input"], toolUse["input"])
	assert.Equal(t, "main.go", input["path"])
}

func TestToolUseInput_EmptyInput_SerializedAsEmptyDictionary(t *testing.T) {
	var captured map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseTextResponse("ok"))
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithAPIKey("test-key"))

	client.StreamMessage(context.Background(), domain.ChatParams{
		Model:  "claude-sonnet-4-6",
		System: "test",
		Messages: []domain.Message{
			domain.NewUserMessage("list files"),
			{
				Role: domain.RoleAssistant,
				Content: []domain.ContentBlock{
					domain.ToolUseContentBlock("tu_2", "list_dir", ""),
				},
			},
			{
				Role: domain.RoleUser,
				Content: []domain.ContentBlock{
					domain.ToolResultContentBlock("tu_2", "file1.go\nfile2.go", false),
				},
			},
		},
		MaxTokens: 1024,
	}, nil)

	msgs := captured["messages"].([]any)
	assistantMsg := msgs[1].(map[string]any)
	blocks := assistantMsg["content"].([]any)
	toolUse := blocks[0].(map[string]any)

	// Empty input must still be a dictionary, not a string.
	input, ok := toolUse["input"].(map[string]any)
	require.True(t, ok, "empty tool_use input must be a dictionary, got %T: %v", toolUse["input"], toolUse["input"])
	assert.Empty(t, input)
}

// --- Tool conversion ---

func TestToolConversion_SendsToolDefinitions(t *testing.T) {
	var captured map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseTextResponse("ok"))
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithAPIKey("test-key"))

	client.StreamMessage(context.Background(), domain.ChatParams{
		Model:    "claude-sonnet-4-6",
		System:   "test",
		Messages: []domain.Message{domain.NewUserMessage("hi")},
		Tools: []domain.ToolDef{
			{
				Name:        "read_file",
				Description: "Read a file",
				InputSchema: map[string]any{
					"properties": map[string]any{
						"path": map[string]any{"type": "string"},
					},
					"required": []string{"path"},
				},
			},
		},
		MaxTokens: 1024,
	}, nil)

	tools := captured["tools"].([]any)
	require.Len(t, tools, 1)
	tool := tools[0].(map[string]any)
	assert.Equal(t, "read_file", tool["name"])
	assert.Equal(t, "Read a file", tool["description"])

	schema := tool["input_schema"].(map[string]any)
	props := schema["properties"].(map[string]any)
	assert.Contains(t, props, "path")

	required := schema["required"].([]any)
	assert.Contains(t, required, "path")
}

func TestToolConversion_EmptyTools(t *testing.T) {
	var captured map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseTextResponse("ok"))
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithAPIKey("test-key"))

	client.StreamMessage(context.Background(), domain.ChatParams{
		Model:     "claude-sonnet-4-6",
		System:    "test",
		Messages:  []domain.Message{domain.NewUserMessage("hi")},
		Tools:     nil,
		MaxTokens: 1024,
	}, nil)

	// No tools field or empty array
	tools, exists := captured["tools"]
	if exists {
		assert.Nil(t, tools)
	}
}

// --- Auth ---

func TestAuth_APIKeySentAsHeader(t *testing.T) {
	var capturedAPIKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAPIKey = r.Header.Get("X-Api-Key")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseTextResponse("ok"))
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithAPIKey("sk-test-key-123"))

	client.StreamMessage(context.Background(), domain.ChatParams{
		Model:     "claude-sonnet-4-6",
		System:    "test",
		Messages:  []domain.Message{domain.NewUserMessage("hi")},
		MaxTokens: 1024,
	}, nil)

	assert.Equal(t, "sk-test-key-123", capturedAPIKey)
}

func TestWithHTTPClient_UsesInjectedTransport(t *testing.T) {
	var transportUsed bool

	customTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		transportUsed = true
		// Forward to a real test server
		return http.DefaultTransport.RoundTrip(req)
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseTextResponse("ok"))
	}))
	defer server.Close()

	customClient := &http.Client{Transport: customTransport}
	client := llm.NewAnthropicClient(
		llm.WithBaseURL(server.URL),
		llm.WithAPIKey("test-key"),
		llm.WithHTTPClient(customClient),
	)

	_, err := client.StreamMessage(
		context.Background(),
		domain.ChatParams{
			Model:     "claude-sonnet-4-6",
			System:    "test",
			Messages:  []domain.Message{domain.NewUserMessage("hi")},
			MaxTokens: 1024,
		},
		nil,
	)

	require.NoError(t, err)
	assert.True(t, transportUsed, "custom HTTP transport should have been used")
}

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestAuth_BearerTokenSentAsAuthorization(t *testing.T) {
	var capturedAuth string
	var capturedAPIKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		capturedAPIKey = r.Header.Get("X-Api-Key")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseTextResponse("ok"))
	}))
	defer server.Close()

	client := llm.NewAnthropicClient(llm.WithBaseURL(server.URL), llm.WithBearerToken("bearer-token-abc"))

	client.StreamMessage(context.Background(), domain.ChatParams{
		Model:     "claude-sonnet-4-6",
		System:    "test",
		Messages:  []domain.Message{domain.NewUserMessage("hi")},
		MaxTokens: 1024,
	}, nil)

	assert.Equal(t, "Bearer bearer-token-abc", capturedAuth)
	assert.Empty(t, capturedAPIKey, "Bearer token should not be sent as X-Api-Key")
}

// --- SSE test helpers ---

func sseTextResponse(deltas ...string) string {
	var sb strings.Builder
	sb.WriteString(`event: message_start
data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-6","stop_reason":null,"usage":{"input_tokens":10,"output_tokens":1}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

`)
	for _, d := range deltas {
		textJSON, _ := json.Marshal(d)
		fmt.Fprintf(&sb, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":%s}}\n\n", textJSON)
	}
	sb.WriteString(`event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`)
	return sb.String()
}

func sseTextResponseWithTokens(text string, inputTokens, outputTokens int) string {
	var sb strings.Builder
	textJSON, _ := json.Marshal(text)
	// input_tokens is reported in message_delta (where consumeStream reads it),
	// not just in message_start
	fmt.Fprintf(&sb, `event: message_start
data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-6","stop_reason":null,"usage":{"input_tokens":%d,"output_tokens":1}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%s}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"input_tokens":%d,"output_tokens":%d}}

event: message_stop
data: {"type":"message_stop"}

`, inputTokens, textJSON, inputTokens, outputTokens)
	return sb.String()
}

func sseToolUseResponse(toolID, toolName, inputJSON string) string {
	var sb strings.Builder
	inputEscaped, _ := json.Marshal(inputJSON)
	fmt.Fprintf(&sb, `event: message_start
data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-6","stop_reason":null,"usage":{"input_tokens":10,"output_tokens":1}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"%s","name":"%s","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":%s}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`, toolID, toolName, inputEscaped)
	return sb.String()
}
