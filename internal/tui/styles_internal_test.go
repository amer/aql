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
	assert.Contains(t, result, "\x1b[7m")
	assert.Contains(t, result, "\x1b[27m")
	plain := stripAnsiString(result)
	plain = stripAnsiString(plain)
	assert.Equal(t, "hello world", plain)
}

func TestHighlightLineRange_EmptyLine(t *testing.T) {
	result := highlightLineRange("", 0, 5)
	assert.Equal(t, "", result)
}

func TestHighlightLineRange_MultiByte(t *testing.T) {
	// ● is 3 bytes in UTF-8 but 1 visible column.
	// Selecting columns 2-7 from "● coder" should get " coder", not garbled bytes.
	line := "● coder"
	result := highlightLineRange(line, 2, 7)
	// Column 0=●, 1=' ', 2='c', 3='o', 4='d', 5='e', 6='r'
	assert.Equal(t, "● \x1b[7mcoder\x1b[27m", result)
}

func TestHighlightLineRange_MultiByteStart(t *testing.T) {
	// Highlight the bullet itself (column 0-1).
	line := "● coder"
	result := highlightLineRange(line, 0, 1)
	assert.Equal(t, "\x1b[7m●\x1b[27m coder", result)
}

func TestHighlightLineRange_BoxDrawing(t *testing.T) {
	// Box-drawing chars like ╭ and ─ are multi-byte.
	// "╭─ AQL" — ╭ is 3 bytes, ─ is 3 bytes, rest is ASCII.
	line := "╭─ AQL"
	result := highlightLineRange(line, 0, 3)
	// Columns: 0=╭ 1=─ 2=' ' 3=A 4=Q 5=L
	assert.Equal(t, "\x1b[7m╭─ \x1b[27mAQL", result)
}

func TestHighlightLineRange_EscParenB(t *testing.T) {
	// lipgloss uses ESC(B as part of reset. It should be skipped, not counted.
	line := "hello\x1b(B\x1b[m world"
	result := highlightLineRange(line, 0, 5)
	assert.Contains(t, result, "\x1b[7m")
	assert.Contains(t, result, "\x1b[27m")
	// Verify the plain text is preserved
	stripped := stripAnsiString(result)
	// stripAnsiString only handles CSI, also strip ESC(B manually
	assert.Contains(t, stripped, "hello")
	assert.Contains(t, stripped, "world")
}

func TestHighlightLineRange_ANSIBeforeFirstChar(t *testing.T) {
	// ANSI code before first visible char — highlight at col 0 should still work
	line := "\x1b[38;5;252mhello\x1b[0m"
	result := highlightLineRange(line, 0, 3)
	assert.Equal(t, "\x1b[38;5;252m\x1b[7mhel\x1b[27mlo\x1b[0m", result)
}
