package tui

import "fmt"

// RenderHeader renders the welcome header.
func RenderHeader(projectPath string, modelName string, width int) string {
	title := HeaderStyle.Render("AQL")
	subtitle := HeaderDimStyle.Render(" — Agent Quorum Loop")
	line1 := title + subtitle

	path := DimStyle.Render(fmt.Sprintf("  %s", projectPath))
	model := DimStyle.Render("  model: ") + StatusBarModelStyle.Render(modelName)

	return line1 + "\n" + path + "\n" + model
}
