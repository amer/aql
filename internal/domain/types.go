package domain

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Provider-agnostic types: Message, ContentBlock, ToolUseBlock, ToolResultBlock
//   - Stream event types: StreamEvent, ToolCallEvent, ToolDoneEvent, TokenUsageEvent
//   - The ChatClient interface (port for LLM providers)
//   - ChatParams, ChatResponse, ChatToolUse — request/response value types
//   - ToolDef, ToolCall, ToolStatus — tool-related value types
//   - ModelInfo — model metadata
//   - Constants shared across packages (BillingHeader, ClaudeCodeBetas)
//   - Helper constructors (NewUserMessage, TextBlock, etc.)
//
// MUST NOT GO HERE:
//   - Anything that imports other internal packages — domain has zero internal dependencies
//   - Implementation logic (no functions with side effects, no I/O)
//   - Anthropic SDK types — this package is provider-agnostic
//   - TUI types (Bubble Tea messages) — those belong in internal/tui/types.go
//   - Mutable state or global variables (except constants)
//
// Q: I need a new type that both agent and tui use. Where?
// A: If it's a domain concept (messages, tools, models), put it here.
//    If it's a TUI display concept, put it in internal/tui/types.go.
//
// Q: Should I add methods to these types?
// A: Only pure helpers (constructors, formatters). No methods with side effects.
//
// Q: Can I add an Anthropic SDK type here?
// A: No. This package must stay provider-agnostic. SDK types belong in internal/llm.
//
// Q: Where do I add a new event type for streaming?
// A: Add a new field to StreamEvent (union style — exactly one field non-nil per event).
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"time"
)

// StreamEvent represents a chunk of output from the agent.
type StreamEvent struct {
	AgentName  string
	Text       string
	Done       bool
	Error      error
	ToolCall   *ToolCallEvent     // non-nil when agent invokes a tool
	ToolDone   *ToolDoneEvent     // non-nil when a tool finishes
	TokenUsage *TokenUsageEvent   // non-nil after each API response with precise token counts
	History    *HistoryAppendMsg  // non-nil when the caller should append a message to history
	Replace    *HistoryReplaceMsg // non-nil when the caller should replace history entirely (compaction)
}

// HistoryAppendMsg tells the caller to append a message to the agent's history.
// Emitted by Run() so that history mutation happens in the caller's goroutine,
// not the agent's streaming goroutine.
type HistoryAppendMsg struct {
	Message Message
}

// HistoryReplaceMsg tells the caller to replace the agent's entire history.
// Emitted after auto-compaction summarizes the conversation.
type HistoryReplaceMsg struct {
	Messages []Message
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

// TokenUsageEvent carries precise token counts from the API response.
type TokenUsageEvent struct {
	InputTokens  int
	OutputTokens int
}

// ModelInfo holds information about an available model from the API.
type ModelInfo struct {
	ID             string
	DisplayName    string
	MaxInputTokens int64
	CreatedAt      time.Time
}

// --- Conversation message types (port-side, provider-agnostic) ---

// MessageRole identifies the sender of a conversation message.
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
)

// Message represents a single message in a conversation history.
type Message struct {
	Role    MessageRole
	Content []ContentBlock
}

// NewUserMessage creates a user message with a single text block.
func NewUserMessage(text string) Message {
	return Message{Role: RoleUser, Content: []ContentBlock{{Text: text}}}
}

// NewAssistantMessage creates an assistant message with a single text block.
func NewAssistantMessage(text string) Message {
	return Message{Role: RoleAssistant, Content: []ContentBlock{{Text: text}}}
}

// ContentBlock is a union type: exactly one field is non-zero.
type ContentBlock struct {
	Text       string
	ToolUse    *ToolUseBlock
	ToolResult *ToolResultBlock
}

// TextBlock creates a text content block.
func TextBlock(text string) ContentBlock {
	return ContentBlock{Text: text}
}

// ToolUseContentBlock creates a tool_use content block.
func ToolUseContentBlock(id, name, input string) ContentBlock {
	return ContentBlock{ToolUse: &ToolUseBlock{ID: id, Name: name, Input: input}}
}

// ToolResultContentBlock creates a tool_result content block.
func ToolResultContentBlock(toolUseID, content string, isError bool) ContentBlock {
	return ContentBlock{ToolResult: &ToolResultBlock{ToolUseID: toolUseID, Content: content, IsError: isError}}
}

// ToolUseBlock represents a tool invocation by the assistant.
type ToolUseBlock struct {
	ID    string
	Name  string
	Input string // raw JSON
}

// ToolResultBlock represents the result of a tool invocation.
type ToolResultBlock struct {
	ToolUseID string
	Content   string
	IsError   bool
}

// --- ChatClient port (provider-agnostic LLM interface) ---

// ChatClient is the port for sending messages to an LLM and receiving
// streamed responses. Implementations (adapters) handle provider-specific
// details like SDK types, auth headers, and streaming protocols.
type ChatClient interface {
	// StreamMessage sends a conversation and streams the response.
	// The onText callback is invoked for each text delta as it arrives.
	// Returns the complete response or an error.
	StreamMessage(ctx context.Context, params ChatParams, onText func(text string)) (*ChatResponse, error)

	// SendMessage sends a conversation and returns the full response (no streaming).
	// Used for internal operations like compaction where streaming isn't needed.
	SendMessage(ctx context.Context, params ChatParams) (*ChatResponse, error)
}

// ChatParams holds the inputs for a ChatClient call.
type ChatParams struct {
	Model        string
	System       string
	Messages     []Message
	Tools        []ToolDef
	MaxTokens    int
	OAuthBilling bool // when true, adapter injects billing headers and enables thinking
}

// ToolDef defines a tool the LLM can invoke.
type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// ChatResponse holds the accumulated result of a streamed LLM response.
type ChatResponse struct {
	TextParts    []string      // ordered text blocks from the response
	ToolUses     []ChatToolUse // tool_use blocks the LLM wants to invoke
	StopReason   string        // "end_turn", "tool_use", etc.
	InputTokens  int
	OutputTokens int
}

// ChatToolUse represents a tool invocation requested by the LLM.
type ChatToolUse struct {
	ID    string
	Name  string
	Input string // raw JSON
}

// BillingHeader is the Claude Code billing header that enables access to
// Opus/Sonnet models via OAuth Console login.
//
// The cc_version / cch values mirror a specific Claude Code release and will
// drift as that client updates. If billing-gated probes start failing with
// 401/403, refresh these by capturing the header the official Claude Code CLI
// sends (cc_version tracks its package version). This is a compatibility
// shim, not a stable API — treat the exact string as disposable.
const BillingHeader = "x-anthropic-billing-header: cc_version=2.1.87.7b6; cc_entrypoint=cli; cch=22c94;"

// ClaudeCodeBetas are the beta feature flags required for Claude Code billing.
const ClaudeCodeBetas = "claude-code-20250219,interleaved-thinking-2025-05-14,effort-2025-11-24"

// ToolStatus represents the execution state of a tool call.
type ToolStatus int

const (
	ToolRunning ToolStatus = iota
	ToolDone
	ToolError
)

// ToolCall represents a tool invocation for display.
type ToolCall struct {
	Name    string
	Content string
	Status  ToolStatus
	ToolID  string
}
