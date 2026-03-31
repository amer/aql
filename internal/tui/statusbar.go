package tui

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - RenderStatusBar — bottom status bar with model name, hints,
//     token count.
//
// MUST NOT GO HERE:
//   - State mutation, token calculations (cost.go), model logic.
// ──────────────────────────────────────────────────────────────────

import (
	"strings"
)

// RenderStatusBar renders the bottom status bar.
// Left: model name, Center: hints, Right: token count
func RenderStatusBar(modelName string, tokenCount int, width int, hints ...string) string {
	tokenText := FormatTokenCountShort(tokenCount) + " tokens"
	left := MutedStyle.Render(modelName)
	right := DimStyle.Render(tokenText)

	center := ""
	if len(hints) > 0 {
		center = DimStyle.Render(strings.Join(hints, " · "))
	}

	leftWidth := len(modelName)
	rightWidth := len(tokenText)
	centerWidth := len(center)
	if centerWidth > 0 {
		centerWidth = len(strings.Join(hints, " · "))
	}
	totalUsed := leftWidth + centerWidth + rightWidth
	remaining := max(width-totalUsed, 2)

	if center == "" {
		return left + strings.Repeat(" ", remaining) + right
	}

	leftGap := max((remaining)/2, 1)
	rightGap := max(remaining-leftGap, 1)
	return left + strings.Repeat(" ", leftGap) + center + strings.Repeat(" ", rightGap) + right
}
