package tui

// RenderPrompt renders the input prompt line.
func RenderPrompt(input string, width int) string {
	cursor := PromptCursor.Render("> ")
	content := cursor + input + "█"
	return PromptStyle.Width(width).Render(content)
}
