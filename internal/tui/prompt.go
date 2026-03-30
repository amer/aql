package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderPrompt renders the input prompt with ❯ prefix.
func RenderPrompt(input string, width int) string {
	cursor := PromptCursor.Render("❯ ")
	return cursor + input + "█"
}

// RenderPromptStreaming renders the prompt area while an agent is responding.
func RenderPromptStreaming(spinnerFrame int, agentName string, width int) string {
	return RenderSpinnerWithType(spinnerFrame, agentName+" is responding...", SpinnerBraille)
}

// RenderPromptArea renders the full prompt area matching Claude Code's layout:
// ─────────────────────────────────── project-name ──
// ❯ input
// ───────────────────────────────────────────────────
func RenderPromptArea(input string, projectName string, width int) string {
	var b strings.Builder

	lineStyle := lipgloss.NewStyle().Foreground(dimColor)
	badgeStyle := lipgloss.NewStyle().Foreground(mutedColor)

	// Top separator: ────────── project-name ──
	badge := badgeStyle.Render(projectName)
	badgeWidth := lipgloss.Width(badge)
	trailWidth := 2
	leadWidth := width - badgeWidth - trailWidth - 2 // 2 spaces around badge
	if leadWidth < 1 {
		leadWidth = 1
	}
	topLine := lineStyle.Render(strings.Repeat("─", leadWidth)) +
		" " + badge + " " +
		lineStyle.Render(strings.Repeat("─", trailWidth))
	b.WriteString(topLine)
	b.WriteString("\n")

	// Prompt line
	cursor := PromptCursor.Render("❯ ")
	b.WriteString(cursor + input + "█")
	b.WriteString("\n")

	// Bottom separator
	b.WriteString(lineStyle.Render(strings.Repeat("─", width)))

	return b.String()
}

// RenderPromptAreaStreaming renders the prompt area during streaming.
func RenderPromptAreaStreaming(spinnerFrame int, agentName string, projectName string, width int, st SpinnerType) string {
	var b strings.Builder

	lineStyle := lipgloss.NewStyle().Foreground(dimColor)
	badgeStyle := lipgloss.NewStyle().Foreground(mutedColor)

	// Top separator with project badge
	badge := badgeStyle.Render(projectName)
	badgeWidth := lipgloss.Width(badge)
	trailWidth := 2
	leadWidth := width - badgeWidth - trailWidth - 2
	if leadWidth < 1 {
		leadWidth = 1
	}
	topLine := lineStyle.Render(strings.Repeat("─", leadWidth)) +
		" " + badge + " " +
		lineStyle.Render(strings.Repeat("─", trailWidth))
	b.WriteString(topLine)
	b.WriteString("\n")

	// Spinner line
	b.WriteString(RenderSpinnerWithType(spinnerFrame, agentName+" is responding...", st))
	b.WriteString("\n")

	// Bottom separator
	b.WriteString(lineStyle.Render(strings.Repeat("─", width)))

	return b.String()
}
