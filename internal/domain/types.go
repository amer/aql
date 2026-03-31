package domain

import "time"

// StreamEvent represents a chunk of output from the agent.
type StreamEvent struct {
	AgentName  string
	Text       string
	Done       bool
	Error      error
	ToolCall   *ToolCallEvent   // non-nil when agent invokes a tool
	ToolDone   *ToolDoneEvent   // non-nil when a tool finishes
	TokenUsage *TokenUsageEvent // non-nil after each API response with precise token counts
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
