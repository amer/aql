package agent_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/amer/aql/internal/agent"
	"github.com/amer/aql/internal/llm"
)

// testClientOpts returns agent options that connect to a test HTTP server.
func testClientOpts(serverURL string) []agent.Option {
	return []agent.Option{
		agent.WithChatClient(llm.NewAnthropicClient(
			llm.WithBaseURL(serverURL),
			llm.WithAPIKey("test-key"),
		)),
	}
}

// testOAuthClientOpts returns agent options for OAuth-style test connections.
func testOAuthClientOpts(serverURL, oauthKey string) []agent.Option {
	return []agent.Option{
		agent.WithChatClient(llm.NewAnthropicClient(
			llm.WithBaseURL(serverURL),
			llm.WithBearerToken(oauthKey),
		)),
		agent.WithOAuth(),
	}
}

// jsonToSSE converts a Messages API JSON response into an SSE stream that
// NewStreaming can parse. This lets existing replay tests keep their JSON
// fixtures while the runner uses streaming.
func jsonToSSE(jsonBody []byte) []byte {
	var msg struct {
		ID         string          `json:"id"`
		Model      string          `json:"model"`
		StopReason string          `json:"stop_reason"`
		Content    json.RawMessage `json:"content"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(jsonBody, &msg); err != nil {
		panic("jsonToSSE: " + err.Error())
	}

	var blocks []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		panic("jsonToSSE content: " + err.Error())
	}

	var sb strings.Builder

	// message_start
	fmt.Fprintf(&sb, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":%q,\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":%q,\"stop_reason\":null,\"usage\":{\"input_tokens\":%d,\"output_tokens\":1}}}\n\n",
		msg.ID, msg.Model, msg.Usage.InputTokens)

	// content blocks
	for i, block := range blocks {
		switch block.Type {
		case "text":
			fmt.Fprintf(&sb, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":%d,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n", i)
			// Emit text as a single delta
			textJSON, _ := json.Marshal(block.Text)
			fmt.Fprintf(&sb, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":%d,\"delta\":{\"type\":\"text_delta\",\"text\":%s}}\n\n", i, textJSON)
			fmt.Fprintf(&sb, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":%d}\n\n", i)

		case "tool_use":
			inputStr := string(block.Input)
			if inputStr == "" || inputStr == "null" {
				inputStr = "{}"
			}
			fmt.Fprintf(&sb, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":%d,\"content_block\":{\"type\":\"tool_use\",\"id\":%q,\"name\":%q,\"input\":{}}}\n\n", i, block.ID, block.Name)
			inputJSON, _ := json.Marshal(inputStr)
			fmt.Fprintf(&sb, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":%d,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":%s}}\n\n", i, inputJSON)
			fmt.Fprintf(&sb, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":%d}\n\n", i)
		}
	}

	// message_delta with stop_reason
	stopReason := msg.StopReason
	if stopReason == "" {
		stopReason = "end_turn"
	}
	fmt.Fprintf(&sb, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":%q,\"stop_sequence\":null},\"usage\":{\"output_tokens\":%d}}\n\n", stopReason, msg.Usage.OutputTokens)

	fmt.Fprintf(&sb, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")

	return []byte(sb.String())
}

// serveSSE is a helper that writes an SSE response with proper headers.
func serveSSE(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(200)
	w.Write(data)
}
