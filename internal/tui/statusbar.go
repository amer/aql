package tui

import (
	"fmt"
	"strings"
)

// AgentStatus represents the state of an agent for display.
type AgentStatus string

const (
	AgentActive  AgentStatus = "active"
	AgentWaiting AgentStatus = "waiting"
	AgentDone    AgentStatus = "done"
	AgentError   AgentStatus = "error"
)

// formatTokenCount formats a token count for display (e.g. "1.5k tokens", "0 tokens").
func formatTokenCount(tokens int) string {
	if tokens == 0 {
		return "0 tokens"
	}
	if tokens < 1000 {
		return fmt.Sprintf("%d tokens", tokens)
	}
	return fmt.Sprintf("%.1fk tokens", float64(tokens)/1000)
}

// RenderStatusBar renders the bottom status bar.
// Left: model name, Right: token count
func RenderStatusBar(modelName string, tokenCount int, width int) string {
	left := MutedStyle.Render(modelName)
	right := DimStyle.Render(formatTokenCount(tokenCount))

	leftWidth := len(modelName)
	rightWidth := len(formatTokenCount(tokenCount))
	gap := max(width-leftWidth-rightWidth, 1)

	return left + strings.Repeat(" ", gap) + right
}
