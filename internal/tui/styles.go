package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Claude Code color palette
	brandColor   = lipgloss.Color("#DA7756") // claude orange
	accentColor  = lipgloss.Color("#5C94F0") // blue accent
	successColor = lipgloss.Color("#5CB85C") // green
	warningColor = lipgloss.Color("#D4A843") // amber
	errorColor   = lipgloss.Color("#D9534F") // red
	dimColor     = lipgloss.Color("#555555") // dim gray
	mutedColor   = lipgloss.Color("#888888") // muted
	textColor    = lipgloss.Color("#D4D4D4") // light text
	brightColor  = lipgloss.Color("#FFFFFF") // bright white
	codeColor    = lipgloss.Color("#7EC8E3") // cyan for code

	// Header / welcome
	HeaderStyle = lipgloss.NewStyle().
			Foreground(brandColor).
			Bold(true)

	HeaderDimStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Status bar (bottom)
	StatusBarModelStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true)

	// User message
	UserInputStyle = lipgloss.NewStyle().
			Foreground(brightColor).
			Bold(true)

	UserPrefixStyle = lipgloss.NewStyle().
			Foreground(brandColor).
			Bold(true)

	// Agent response
	AgentHeaderActive = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true)

	AgentHeaderWaiting = lipgloss.NewStyle().
				Foreground(warningColor).
				Bold(true)

	AgentHeaderDone = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	AgentHeaderError = lipgloss.NewStyle().
				Foreground(errorColor).
				Bold(true)

	AgentBody = lipgloss.NewStyle().
			Foreground(textColor).
			PaddingLeft(2)

	// Tool call block — Claude Code style
	ToolBlockBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dimColor).
			MarginLeft(2).
			PaddingLeft(1).
			PaddingRight(1)

	ToolHeaderStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Bold(true)

	ToolContentStyle = lipgloss.NewStyle().
				Foreground(textColor)

	ToolLabelStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)

	ToolStatusRunning = lipgloss.NewStyle().
				Foreground(warningColor)

	ToolStatusDone = lipgloss.NewStyle().
			Foreground(successColor)

	ToolStatusError = lipgloss.NewStyle().
			Foreground(errorColor)

	// Prompt input
	PromptCursor = lipgloss.NewStyle().
			Foreground(brandColor).
			Bold(true)

	// Code
	CodeBlockStyle = lipgloss.NewStyle().
			Foreground(codeColor)

	CodeInlineStyle = lipgloss.NewStyle().
			Foreground(codeColor)

	// General
	DimStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	MutedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	BoldStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor)

	// Command palette
	PaletteBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(dimColor).
				PaddingLeft(1).
				PaddingRight(1)

	PaletteSelectedStyle = lipgloss.NewStyle().
				Foreground(brandColor).
				Bold(true)

	// Spinner
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(brandColor)
)
