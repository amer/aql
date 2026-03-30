package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHighlightLineRange_PlainText(t *testing.T) {
	result := highlightLineRange("hello world", 0, 5)
	assert.Equal(t, "\x1b[7mhello\x1b[27m world", result)
}

func TestHighlightLineRange_MiddleRange(t *testing.T) {
	result := highlightLineRange("hello world", 6, 11)
	assert.Equal(t, "hello \x1b[7mworld\x1b[27m", result)
}

func TestHighlightLineRange_ToEndOfLine(t *testing.T) {
	result := highlightLineRange("hello", 2, -1)
	assert.Equal(t, "he\x1b[7mllo\x1b[27m", result)
}

func TestHighlightLineRange_WithANSI(t *testing.T) {
	// Line with ANSI color codes — highlight should skip escape sequences
	line := "\x1b[1mhello\x1b[0m world"
	result := highlightLineRange(line, 0, 5)
	// The ANSI codes pass through; reverse video wraps visible chars 0-4
	assert.Contains(t, result, "\x1b[7m")
	assert.Contains(t, result, "\x1b[27m")
	// Strip both highlight and original ANSI to verify text is intact
	plain := stripAnsiString(result)
	// Also strip reverse video
	plain = stripAnsiString(plain)
	assert.Equal(t, "hello world", plain)
}

func TestHighlightLineRange_EmptyLine(t *testing.T) {
	result := highlightLineRange("", 0, 5)
	assert.Equal(t, "", result)
}
