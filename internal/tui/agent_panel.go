package tui

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - AgentPanelData struct, RenderAgentHeader — agent name with
//     status indicator, RenderToolBlock — bordered tool call block,
//     RenderAgentPanel, RenderUserMessage.
//
// MUST NOT GO HERE:
//   - Message handling, state mutation, agent imports.
// ──────────────────────────────────────────────────────────────────

import (
	"fmt"
	"strings"

	"github.com/amer/aql/internal/domain"
)

// AgentPanelData holds the data needed to render an agent panel.
type AgentPanelData struct {
	Name      string
	Status    AgentStatus
	Output    string
	ToolCalls []domain.ToolCall
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
func RenderToolBlock(tc domain.ToolCall) string {
	// Status indicator
	var statusIndicator string
	switch tc.Status {
	case domain.ToolRunning:
		statusIndicator = ToolStatusRunning.Render("⟳ ")
	case domain.ToolDone:
		statusIndicator = ToolStatusDone.Render("✓ ")
	case domain.ToolError:
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
	var result strings.Builder

	result.WriteString(RenderAgentHeader(data.Name, data.Status) + "\n")

	if data.Output != "" {
		result.WriteString(AgentBody.Render(data.Output) + "\n")
	}

	for _, tc := range data.ToolCalls {
		result.WriteString(RenderToolBlock(tc) + "\n")
	}

	if data.StatusMsg != "" {
		result.WriteString(DimStyle.Render("  "+data.StatusMsg) + "\n")
	}

	return result.String()
}

// RenderUserMessage renders a user input message in Claude Code style.
func RenderUserMessage(content string) string {
	prefix := UserPrefixStyle.Render("> ")
	text := UserInputStyle.Render(content)
	return fmt.Sprintf("\n%s%s\n", prefix, text)
}
