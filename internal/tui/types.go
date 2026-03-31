package tui

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - AgentStatus enum, ChatEntryType/ChatEntry — chat log item
//     types, all Bubble Tea message types (AgentStreamStartMsg,
//     AgentStreamDeltaMsg, AgentStreamDoneMsg, AgentStreamErrorMsg,
//     AgentStatusMsg, AgentToolCallMsg, AgentAskUserMsg,
//     ModelSelectedMsg, ModelsLoadedMsg, BashResultMsg,
//     CompactDoneMsg, TokenUsageMsg, TokenCountMsg),
//     BashFunc/SubmitFunc callback types.
//
// MUST NOT GO HERE:
//   - Message handling logic (handlers.go), rendering, agent
//     imports. Messages are value types — immutable data carriers.
//
// Q: Should I add a new TUI message?
// A: Define it here as a value type struct. Handle it in
//    handlers.go's handleMsg(). If it originates from a domain
//    event, translate it in stream/adapter.go.
//
// Q: Can messages contain channels?
// A: Only AgentAskUserMsg has a ResponseCh. This is the exception,
//    not the norm.
// ──────────────────────────────────────────────────────────────────

import (
	"github.com/amer/aql/internal/diff"
	"github.com/amer/aql/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

// AgentStatus represents the state of an agent for display.
type AgentStatus string

const (
	AgentActive  AgentStatus = "active"
	AgentWaiting AgentStatus = "waiting"
	AgentDone    AgentStatus = "done"
	AgentError   AgentStatus = "error"
)

// ChatEntryType identifies the kind of chat entry.
type ChatEntryType int

const (
	EntryUserInput ChatEntryType = iota
	EntryAgentText
	EntryAgentTool
	EntryAgentStatus
)

// ChatEntry represents a single item in the scrolling chat log.
type ChatEntry struct {
	Type      ChatEntryType
	AgentName string
	Content   string
	ToolCall  *domain.ToolCall
	Status    AgentStatus
}

// AgentOutputMsg is sent when an agent produces new output.
type AgentOutputMsg struct {
	AgentName string
	Output    string
}

// AgentStreamStartMsg is sent when the agent starts processing (before API call).
type AgentStreamStartMsg struct {
	AgentName string
}

// AgentStreamDeltaMsg is sent for each streamed text chunk.
type AgentStreamDeltaMsg struct {
	AgentName string
	Delta     string
}

// AgentStreamDoneMsg is sent when streaming is complete.
type AgentStreamDoneMsg struct {
	AgentName string
}

// AgentStreamErrorMsg is sent when streaming encounters an error.
type AgentStreamErrorMsg struct {
	AgentName string
	Error     error
}

// AgentStatusMsg is sent when an agent's status changes.
type AgentStatusMsg struct {
	AgentName string
	Status    AgentStatus
	StatusMsg string
}

// AgentToolCallMsg is sent when an agent invokes a tool.
type AgentToolCallMsg struct {
	AgentName string
	ToolCall  domain.ToolCall
}

// AgentAskUserMsg is sent when the agent uses ask_user to request user input.
// The TUI shows the question and sends the user's answer back via ResponseCh.
type AgentAskUserMsg struct {
	AgentName  string
	Question   string
	ResponseCh chan<- string
}

// ModelSelectedMsg is emitted when the user selects a model via /model <name>.
// The main app should persist this selection and reconfigure agents.
type ModelSelectedMsg struct {
	Model string
}

// ModelsLoadedMsg is sent when background model probing completes.
type ModelsLoadedMsg struct {
	Tiers []ModelTier
}

// BashResultMsg is sent when a ! bash command completes.
type BashResultMsg struct {
	Command string
	Output  string
	Error   error
}

// CompactDoneMsg is sent when context compaction completes.
type CompactDoneMsg struct {
	Summary string
	Err     error
}

// TokenUsageMsg is sent with precise token counts from the API response.
type TokenUsageMsg = domain.TokenUsageEvent

// TokenCountMsg updates the token count in the status bar.
type TokenCountMsg struct {
	Count int
}

// DiffResultMsg is sent when the diff runner completes.
type DiffResultMsg struct {
	Files []diff.DiffFile
	Stats diff.DiffStats
	Err   error
}

// BashFunc executes a shell command and returns a tea.Cmd with the result.
// Set by the main app to provide actual shell execution.
type BashFunc func(command string) tea.Cmd

// SubmitFunc is called when the user submits input. It should return a tea.Cmd
// that kicks off agent processing.
type SubmitFunc func(input string) tea.Cmd
