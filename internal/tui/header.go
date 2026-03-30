package tui

import "fmt"

// RenderHeader renders the Claude Code-style welcome header.
func RenderHeader(projectPath string, modelName string, width int) string {
	title := HeaderStyle.Render("╭─ AQL")
	subtitle := HeaderDimStyle.Render(" — Agent Quorum Loop")
	line1 := title + subtitle

	path := DimStyle.Render(fmt.Sprintf("│ %s", projectPath))
	model := DimStyle.Render(fmt.Sprintf("│ model: ")) + StatusBarModelStyle.Render(modelName)
	bottom := DimStyle.Render("╰─")

	return line1 + "\n" + path + "\n" + model + "\n" + bottom
}
