package tui

// RenderPrompt renders the Claude Code-style input prompt.
func RenderPrompt(input string, width int) string {
	cursor := PromptCursor.Render("> ")
	content := cursor + input + "█"
	return PromptStyle.Width(width).Render(content)
}

// RenderPromptStreaming renders the prompt area while an agent is responding.
func RenderPromptStreaming(spinnerFrame int, agentName string, width int) string {
	return PromptStyle.Width(width).Render(
		RenderSpinner(spinnerFrame, agentName+" is responding..."),
	)
}
