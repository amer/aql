package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderPrompt renders the input prompt with ❯ prefix.
// The input parameter should already include cursor rendering.
func RenderPrompt(input string, width int) string {
	cursor := PromptCursor.Render("❯ ")
	return cursor + input
}

// renderPromptFrame renders the prompt area with separator bars and a project badge.
// content is the middle line (prompt input or spinner).
func renderPromptFrame(content string, projectName string, width int) string {
	var b strings.Builder

	lineStyle := PromptLineStyle
	badgeStyle := PromptBadgeStyle

	// Top bar: ────────── project-name ──
	badge := badgeStyle.Render(projectName)
	badgeWidth := lipgloss.Width(badge)
	trailWidth := 2
	leadWidth := max(width-badgeWidth-trailWidth-2, 1)
	b.WriteString(lineStyle.Render(strings.Repeat("─", leadWidth)))
	b.WriteString(" ")
	b.WriteString(badge)
	b.WriteString(" ")
	b.WriteString(lineStyle.Render(strings.Repeat("─", trailWidth)))
	b.WriteString("\n")

	// Content line
	b.WriteString(content)
	b.WriteString("\n")

	// Bottom bar
	b.WriteString(lineStyle.Render(strings.Repeat("─", width)))

	return b.String()
}

// RenderPromptArea renders the full prompt area matching Claude Code's layout.
// The input parameter should already include cursor rendering (from InputBuffer.RenderWithCursor).
func RenderPromptArea(input string, projectName string, width int) string {
	cursor := PromptCursor.Render("❯ ")
	return renderPromptFrame(cursor+input, projectName, width)
}

// RenderPromptAreaStreaming renders the prompt area during streaming.
func RenderPromptAreaStreaming(spinnerFrame int, agentName string, projectName string, width int, st SpinnerType) string {
	spinner := RenderSpinnerWithType(spinnerFrame, agentName+" is responding...", st)
	return renderPromptFrame(spinner, projectName, width)
}
