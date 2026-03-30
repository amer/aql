package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultilineShiftEnterAddsNewline(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyKey(m, "h")
	m = applyKey(m, "i")

	// Shift+Enter should add a newline, not submit
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	m = updated.(tui.Model)

	assert.Nil(t, cmd)
	assert.Contains(t, m.Input(), "\n")
	assert.Len(t, m.Chat(), 0, "should not submit on alt+enter")
}

func TestMultilineEnterSubmitsMultilineInput(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyKey(m, "h")
	m = applyKey(m, "i")

	// Add a newline
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	m = updated.(tui.Model)

	m = applyKey(m, "b")
	m = applyKey(m, "y")
	m = applyKey(m, "e")

	// Regular Enter submits the multiline content
	m = applyKey(m, "enter")

	require.Len(t, m.Chat(), 1)
	assert.Equal(t, "hi\nbye", m.Chat()[0].Content)
	assert.Equal(t, "", m.Input())
}

func TestMultilineBackspaceRemovesNewline(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyKey(m, "a")

	// Add newline
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	m = updated.(tui.Model)

	// Backspace should remove the newline
	m = applyKey(m, "backspace")
	assert.Equal(t, "a", m.Input())
}

func TestMultilineRenderPromptShowsMultipleLines(t *testing.T) {
	result := tui.RenderPrompt("line1\nline2", 60)
	assert.Contains(t, result, "line1")
	assert.Contains(t, result, "line2")
}

func TestMultilineInputCursorPosition(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyKey(m, "a")
	m = applyKey(m, "b")

	// Add newline
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	m = updated.(tui.Model)

	m = applyKey(m, "c")
	assert.Equal(t, "ab\nc", m.Input())
}
