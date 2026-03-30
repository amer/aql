package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// AgentStatus represents the state of an agent for display.
type AgentStatus string

const (
	AgentActive  AgentStatus = "active"
	AgentWaiting AgentStatus = "waiting"
	AgentDone    AgentStatus = "done"
	AgentError   AgentStatus = "error"
)

// maxContextTokens is the assumed context window size for auto-compact percentage.
const maxContextTokens = 200000

// RenderStatusBar renders the Claude Code-style bottom status bar.
// Left: "▸▸ agents on (shift+tab to cycle)"
// Right: "N% until auto-compact"
func RenderStatusBar(modelName string, tokenCount int, width int) string {
	arrowStyle := lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	arrows := arrowStyle.Render("▸▸")

	left := arrows + " " + DimStyle.Render("agents on") + " " + MutedStyle.Render("(shift+tab to cycle)")

	pct := tokenCount * 100 / maxContextTokens
	remaining := 100 - pct
	if remaining < 0 {
		remaining = 0
	}
	right := DimStyle.Render(fmt.Sprintf("%d%% until auto-compact", remaining))

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := width - leftWidth - rightWidth
	if gap < 1 {
		gap = 1
	}

	return left + strings.Repeat(" ", gap) + right
}

func formatTokens(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fm", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
