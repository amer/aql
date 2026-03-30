package tui

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripAnsiString removes ANSI escape sequences from a string.
func stripAnsiString(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

// highlightLineRange applies ANSI reverse-video (\x1b[7m) to characters at
// visible columns [fromCol, toCol) in a line that may contain ANSI escapes.
// If toCol < 0, highlights to end of line.
// Columns are counted per rune (not byte) so multi-byte characters like ● or ╭
// each count as one column.
func highlightLineRange(line string, fromCol, toCol int) string {
	var result strings.Builder
	result.Grow(len(line) + 20)
	col := 0
	inHighlight := false
	i := 0

	for i < len(line) {
		// Skip all ANSI escape sequences without counting them as columns.
		if line[i] == '\x1b' && i+1 < len(line) {
			next := line[i+1]
			switch {
			case next == '[':
				// CSI sequence: ESC [ <params> <letter>
				j := i + 2
				for j < len(line) && !((line[j] >= 'A' && line[j] <= 'Z') || (line[j] >= 'a' && line[j] <= 'z')) {
					j++
				}
				if j < len(line) {
					j++ // include terminating letter
				}
				result.WriteString(line[i:j])
				i = j
				continue
			case next >= 0x20 && next <= 0x2F:
				// Intermediate-byte sequence: ESC <intermediate>+ <final>
				// Covers ESC(B, ESC## etc. used by lipgloss resets.
				j := i + 1
				for j < len(line) && line[j] >= 0x20 && line[j] <= 0x2F {
					j++
				}
				if j < len(line) {
					j++ // include final byte
				}
				result.WriteString(line[i:j])
				i = j
				continue
			case next == ']':
				// OSC sequence: ESC ] ... BEL/ST
				j := i + 2
				for j < len(line) && line[j] != '\x07' {
					if line[j] == '\x1b' && j+1 < len(line) && line[j+1] == '\\' {
						j += 2
						break
					}
					j++
				}
				if j < len(line) && line[j] == '\x07' {
					j++
				}
				result.WriteString(line[i:j])
				i = j
				continue
			default:
				// Two-byte escape (e.g. ESC 7, ESC 8)
				result.WriteString(line[i : i+2])
				i += 2
				continue
			}
		}

		// Check highlight boundaries before writing the rune.
		if col == fromCol && !inHighlight {
			result.WriteString("\x1b[7m")
			inHighlight = true
		}
		if toCol >= 0 && col == toCol && inHighlight {
			result.WriteString("\x1b[27m")
			inHighlight = false
		}

		// Write one complete UTF-8 rune (not byte).
		_, size := utf8.DecodeRuneInString(line[i:])
		result.WriteString(line[i : i+size])
		i += size
		col++
	}

	if inHighlight {
		result.WriteString("\x1b[27m")
	}
	return result.String()
}

var (
	// Tokyo Night Storm color palette
	brandColor   = lipgloss.Color("#ff9e64") // orange
	accentColor  = lipgloss.Color("#7aa2f7") // blue
	successColor = lipgloss.Color("#9ece6a") // green
	warningColor = lipgloss.Color("#e0af68") // yellow
	errorColor   = lipgloss.Color("#f7768e") // red
	dimColor     = lipgloss.Color("#545c7e") // dark3
	mutedColor   = lipgloss.Color("#737aa2") // dark5
	textColor    = lipgloss.Color("#a9b1d6") // foreground dark
	brightColor  = lipgloss.Color("#c0caf5") // foreground
	codeColor    = lipgloss.Color("#7dcfff") // cyan

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

	// Accent (for streaming indicator ✦)
	AccentStyle = lipgloss.NewStyle().
			Foreground(brandColor).
			Bold(true)

	// Spinner
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(brandColor)
)
