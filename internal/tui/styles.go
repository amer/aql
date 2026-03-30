package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED") // purple
	secondaryColor = lipgloss.Color("#6B7280") // gray
	successColor   = lipgloss.Color("#10B981") // green
	warningColor   = lipgloss.Color("#F59E0B") // amber
	errorColor     = lipgloss.Color("#EF4444") // red
	dimColor       = lipgloss.Color("#4B5563") // dim gray
	textColor      = lipgloss.Color("#E5E7EB") // light gray

	// Status bar
	StatusBarStyle = lipgloss.NewStyle().
			Background(primaryColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true)

	// Agent panel
	AgentHeaderActive = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)

	AgentHeaderWaiting = lipgloss.NewStyle().
				Foreground(warningColor).
				Bold(true)

	AgentHeaderDone = lipgloss.NewStyle().
			Foreground(dimColor).
			Bold(true)

	AgentHeaderError = lipgloss.NewStyle().
				Foreground(errorColor).
				Bold(true)

	AgentBody = lipgloss.NewStyle().
			Foreground(textColor).
			PaddingLeft(2)

	// Tool call block
	ToolBlockStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondaryColor).
			PaddingLeft(1).
			PaddingRight(1).
			MarginLeft(2)

	ToolLabelStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Italic(true)

	// Prompt
	PromptStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(dimColor).
			PaddingLeft(1)

	PromptCursor = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	// User input
	UserInputStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	// Dim text
	DimStyle = lipgloss.NewStyle().
			Foreground(dimColor)
)
