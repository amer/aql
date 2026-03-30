package tui

import "fmt"

// ToolCall represents a tool invocation to display.
type ToolCall struct {
	Name    string
	Content string
}

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

// RenderToolBlock renders a tool call as a bordered block.
func RenderToolBlock(tc ToolCall) string {
	label := ToolLabelStyle.Render(fmt.Sprintf("tool: %s", tc.Name))
	content := tc.Content
	if content == "" {
		content = "(no output)"
	}
	return ToolBlockStyle.Render(label + "\n" + content)
}

// RenderAgentPanel renders a complete agent panel with header, output, and tool blocks.
func RenderAgentPanel(data AgentPanelData) string {
	var result string

	result += RenderAgentHeader(data.Name, data.Status) + "\n"

	if data.Output != "" {
		result += AgentBody.Render("> "+data.Output) + "\n"
	}

	for _, tc := range data.ToolCalls {
		result += RenderToolBlock(tc) + "\n"
	}

	if data.StatusMsg != "" {
		result += DimStyle.Render("  "+data.StatusMsg) + "\n"
	}

	return result
}
