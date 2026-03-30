package tui

import "fmt"

// AgentStatus represents the state of an agent for display.
type AgentStatus string

const (
	AgentActive  AgentStatus = "active"
	AgentWaiting AgentStatus = "waiting"
	AgentDone    AgentStatus = "done"
	AgentError   AgentStatus = "error"
)

// RenderStatusBar renders the bottom status bar with model and token info.
func RenderStatusBar(modelName string, tokenCount int, width int) string {
	model := StatusBarModelStyle.Render(modelName)
	tokens := StatusBarTokenStyle.Render(fmt.Sprintf(" · %s tokens", formatTokens(tokenCount)))
	sep := DimStyle.Render(" │ ")
	hint := DimStyle.Render("/exit to quit · ctrl+c to cancel")

	return StatusBarStyle.Render(model + tokens + sep + hint)
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
