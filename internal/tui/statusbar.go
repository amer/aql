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

// RenderStatusBar renders the top status bar with workflow name and agent count.
func RenderStatusBar(workflowName string, agentCount int, width int) string {
	left := fmt.Sprintf(" AQL — %s", workflowName)
	right := fmt.Sprintf("[%d agents] ", agentCount)
	spaces := width - len(left) - len(right)
	if spaces < 1 {
		spaces = 1
	}
	content := left + repeatChar(' ', spaces) + right
	return StatusBarStyle.Width(width).Render(content)
}

func repeatChar(c byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}
