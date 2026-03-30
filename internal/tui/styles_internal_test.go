package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHighlightLineRange_PlainText(t *testing.T) {
	result := highlightLineRange("hello world", 0, 5)
	assert.Equal(t, selHighlightOn+"hello"+selHighlightOff+" world", result)
}

func TestHighlightLineRange_MiddleRange(t *testing.T) {
	result := highlightLineRange("hello world", 6, 11)
	assert.Equal(t, "hello "+selHighlightOn+"world"+selHighlightOff, result)
}

func TestHighlightLineRange_ToEndOfLine(t *testing.T) {
	result := highlightLineRange("hello", 2, -1)
	assert.Equal(t, "he"+selHighlightOn+"llo"+selHighlightOff, result)
}

func TestHighlightLineRange_WithANSI(t *testing.T) {
	line := "\x1b[1mhello\x1b[0m world"
	result := highlightLineRange(line, 0, 5)
	assert.Contains(t, result, selHighlightOn)
	assert.Contains(t, result, selHighlightOff)
	plain := stripAnsiString(result)
	assert.Equal(t, "hello world", plain)
}

func TestHighlightLineRange_EmptyLine(t *testing.T) {
	result := highlightLineRange("", 0, 5)
	assert.Equal(t, "", result)
}

func TestHighlightLineRange_MultiByte(t *testing.T) {
	line := "● coder"
	result := highlightLineRange(line, 2, 7)
	assert.Equal(t, "● "+selHighlightOn+"coder"+selHighlightOff, result)
}

func TestHighlightLineRange_MultiByteStart(t *testing.T) {
	line := "● coder"
	result := highlightLineRange(line, 0, 1)
	assert.Equal(t, selHighlightOn+"●"+selHighlightOff+" coder", result)
}

func TestHighlightLineRange_BoxDrawing(t *testing.T) {
	line := "╭─ AQL"
	result := highlightLineRange(line, 0, 3)
	assert.Equal(t, selHighlightOn+"╭─ "+selHighlightOff+"AQL", result)
}

func TestHighlightLineRange_EscParenB(t *testing.T) {
	line := "hello\x1b(B\x1b[m world"
	result := highlightLineRange(line, 0, 5)
	assert.Contains(t, result, selHighlightOn)
	assert.Contains(t, result, selHighlightOff)
}

func TestHighlightLineRange_ANSIBeforeFirstChar(t *testing.T) {
	line := "\x1b[38;5;252mhello\x1b[0m"
	result := highlightLineRange(line, 0, 3)
	// After initial CSI, highlight starts, then after reset CSI, highlight re-applies
	assert.Contains(t, result, selHighlightOn)
	plain := stripAnsiString(result)
	assert.Equal(t, "hello", plain)
}

func TestHighlightLineRange_SurvivesResets(t *testing.T) {
	// Real lipgloss output has \x1b[0m resets between styled spans.
	// Background color must survive these resets — re-applied after each SGR.
	line := "\x1b[38;5;252mhello\x1b[0m\x1b[38;5;252m world\x1b[0m"
	result := highlightLineRange(line, 0, 11)

	plain := stripAnsiString(result)
	assert.Equal(t, "hello world", plain, "text content should be preserved")

	// Count highlight-on sequences — should be more than 1 due to re-application
	onCount := strings.Count(result, selHighlightOn)
	assert.GreaterOrEqual(t, onCount, 2,
		"highlight should be re-applied after resets: %q", result)
}

// --- stripAnsiString tests ---

func TestStripAnsiString_CSI(t *testing.T) {
	assert.Equal(t, "hello", stripAnsiString("\x1b[1mhello\x1b[0m"))
}

func TestStripAnsiString_EscParenB(t *testing.T) {
	assert.Equal(t, "hello world", stripAnsiString("hello\x1b(B\x1b[m world"))
}

func TestStripAnsiString_SelectionHighlight(t *testing.T) {
	assert.Equal(t, "hello", stripAnsiString(selHighlightOn+"hello"+selHighlightOff))
}

func TestStripAnsiString_Mixed(t *testing.T) {
	line := "\x1b[38;5;252m\x1b[1mhello\x1b(B\x1b[m world\x1b[0m"
	assert.Equal(t, "hello world", stripAnsiString(line))
}

func TestStripAnsiString_PreservesMultiByte(t *testing.T) {
	assert.Equal(t, "● coder", stripAnsiString("\x1b[1m●\x1b[0m coder"))
}

// --- computeViewLines: stripped lines should be pure text ---

func TestComputeViewLines_NoPrintableEscapes(t *testing.T) {
	m := NewModel("test", []string{"coder"}, nil)
	m.width = 80
	m.height = 24
	m.chat = append(m.chat, ChatEntry{
		Type:      EntryAgentText,
		AgentName: "coder",
		Content:   "hello world",
	})
	m.computeViewLines()

	for i, line := range m.viewLines {
		assert.False(t, strings.Contains(line, "\x1b"),
			"viewLine[%d] should not contain ESC: %q", i, line)
	}
}
