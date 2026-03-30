package tui

import "fmt"

// ToolCall represents a tool invocation to display.
type ToolCall struct {
	Name    string
	Content string
	Status  ToolStatus
	ToolID  string
}

// ToolStatus represents the execution state of a tool call.
type ToolStatus int

const (
	ToolRunning ToolStatus = iota
	ToolDone
	ToolError
)

// AgentPanelData holds the data needed to render an agent panel.
type AgentPanelData struct {
	Name      string
	Status    AgentStatus
	Output    string
	ToolCalls []ToolCall
	StatusMsg string
}

// RenderAgentHeader renders the agent name with a status indicator.
func RenderAgentHeader(name string, status AgentStatus) string {
	var indicator string
	var style = AgentHeaderActive

	switch status {
	case AgentActive:
		indicator = "● "
		style = AgentHeaderActive
	case AgentWaiting:
		indicator = "○ "
		style = AgentHeaderWaiting
	case AgentDone:
		indicator = "✓ "
		style = AgentHeaderDone
	case AgentError:
		indicator = "✗ "
		style = AgentHeaderError
	}

	return style.Render(indicator + name)
}

// RenderToolBlock renders a tool call as a Claude Code-style bordered block.
func RenderToolBlock(tc ToolCall) string {
	// Status indicator
	var statusIndicator string
	switch tc.Status {
	case ToolRunning:
		statusIndicator = ToolStatusRunning.Render("⟳ ")
	case ToolDone:
		statusIndicator = ToolStatusDone.Render("✓ ")
	case ToolError:
		statusIndicator = ToolStatusError.Render("✗ ")
	default:
		statusIndicator = ToolStatusDone.Render("✓ ")
	}

	header := statusIndicator + ToolHeaderStyle.Render(tc.Name)

	content := tc.Content
	if content == "" {
		content = DimStyle.Render("(no output)")
	} else {
		content = ToolContentStyle.Render(content)
	}

	return ToolBlockBorder.Render(header + "\n" + content)
}

// RenderAgentPanel renders a complete agent panel.
func RenderAgentPanel(data AgentPanelData) string {
	var result string

	result += RenderAgentHeader(data.Name, data.Status) + "\n"

	if data.Output != "" {
		result += AgentBody.Render(data.Output) + "\n"
	}

	for _, tc := range data.ToolCalls {
		result += RenderToolBlock(tc) + "\n"
	}

	if data.StatusMsg != "" {
		result += DimStyle.Render("  "+data.StatusMsg) + "\n"
	}

	return result
}

// RenderUserMessage renders a user input message in Claude Code style.
func RenderUserMessage(content string) string {
	prefix := UserPrefixStyle.Render("> ")
	text := UserInputStyle.Render(content)
	return fmt.Sprintf("\n%s%s\n", prefix, text)
}
