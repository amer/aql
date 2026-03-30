package tui

import "github.com/charmbracelet/lipgloss"

// RenderPrompt renders the input prompt with ❯ prefix.
// The input parameter should already include cursor rendering.
func RenderPrompt(input string, width int) string {
	cursor := PromptCursor.Render("❯ ")
	return cursor + input
}

// renderPromptFrame renders the prompt area with a project badge.
// content is the middle line (prompt input or spinner).
func renderPromptFrame(content string, projectName string, width int) string {
	badgeStyle := lipgloss.NewStyle().Foreground(mutedColor)
	return badgeStyle.Render(projectName) + "\n" + content
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
