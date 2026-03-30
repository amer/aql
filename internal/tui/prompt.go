package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderPrompt renders the Claude Code-style input prompt with `) ` prefix.
func RenderPrompt(input string, width int) string {
	cursor := PromptCursor.Render(") ")
	content := cursor + input + "█"
	return content
}

// RenderPromptStreaming renders the prompt area while an agent is responding.
func RenderPromptStreaming(spinnerFrame int, agentName string, width int) string {
	return RenderSpinner(spinnerFrame, agentName+" is responding...")
}

// RenderPromptArea renders the full prompt area with teal separator lines
// and a right-aligned project badge, matching Claude Code's layout.
func RenderPromptArea(input string, projectName string, width int) string {
	var b strings.Builder

	lineStyle := lipgloss.NewStyle().Foreground(accentColor)

	// Top separator with right-aligned project badge
	badge := PromptBadgeStyle.Render(projectName)
	badgeWidth := lipgloss.Width(badge)
	lineWidth := width - badgeWidth - 1
	if lineWidth < 1 {
		lineWidth = 1
	}
	topLine := lineStyle.Render(strings.Repeat("─", lineWidth)) + " " + badge
	b.WriteString(topLine)
	b.WriteString("\n")

	// Prompt line
	cursor := PromptCursor.Render(") ")
	b.WriteString(cursor + input + "█")
	b.WriteString("\n")

	// Bottom separator
	b.WriteString(lineStyle.Render(strings.Repeat("─", width)))

	return b.String()
}

// RenderPromptAreaStreaming renders the prompt area during streaming.
func RenderPromptAreaStreaming(spinnerFrame int, agentName string, projectName string, width int) string {
	var b strings.Builder

	lineStyle := lipgloss.NewStyle().Foreground(accentColor)

	// Top separator with right-aligned project badge
	badge := PromptBadgeStyle.Render(projectName)
	badgeWidth := lipgloss.Width(badge)
	lineWidth := width - badgeWidth - 1
	if lineWidth < 1 {
		lineWidth = 1
	}
	topLine := lineStyle.Render(strings.Repeat("─", lineWidth)) + " " + badge
	b.WriteString(topLine)
	b.WriteString("\n")

	// Spinner line
	b.WriteString(RenderSpinner(spinnerFrame, agentName+" is responding..."))
	b.WriteString("\n")

	// Bottom separator
	b.WriteString(lineStyle.Render(strings.Repeat("─", width)))

	return b.String()
}
