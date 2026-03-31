package tui

import (
	"strings"
)

// RenderStatusBar renders the bottom status bar.
// Left: model name, Right: token count
func RenderStatusBar(modelName string, tokenCount int, width int) string {
	tokenText := FormatTokenCountShort(tokenCount) + " tokens"
	left := MutedStyle.Render(modelName)
	right := DimStyle.Render(tokenText)

	leftWidth := len(modelName)
	rightWidth := len(tokenText)
	gap := max(width-leftWidth-rightWidth, 1)

	return left + strings.Repeat(" ", gap) + right
}
