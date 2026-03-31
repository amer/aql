package tui_test

import (
	"fmt"
	"regexp"

	"github.com/amer/aql/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripAnsi removes ANSI escape codes from a string for test assertions.
func stripAnsi(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// applyKey sends a key press to the model and returns the updated model.
func applyKey(m tui.Model, key string) tui.Model {
	var msg tea.Msg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "backspace":
		msg = tea.KeyMsg{Type: tea.KeyBackspace}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		msg = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		msg = tea.KeyMsg{Type: tea.KeyRight}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEscape}
	case "pgup":
		msg = tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		msg = tea.KeyMsg{Type: tea.KeyPgDown}
	case "shift+up":
		msg = tea.KeyMsg{Type: tea.KeyShiftUp}
	case "shift+down":
		msg = tea.KeyMsg{Type: tea.KeyShiftDown}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, _ := m.Update(msg)
	return updated.(tui.Model)
}

// applyMsg sends a tea.Msg to the model and returns the updated model.
func applyMsg(m tui.Model, msg tea.Msg) tui.Model {
	updated, _ := m.Update(msg)
	return updated.(tui.Model)
}

// applyMsgCmd sends a tea.Msg and returns both the updated model and command.
func applyMsgCmd(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(tui.Model), cmd
}

// typeString types each rune of s as individual key presses.
func typeString(m tui.Model, s string) tui.Model {
	for _, c := range s {
		m = applyKey(m, string(c))
	}
	return m
}

// applyPaste simulates a bracketed paste event from the terminal.
func applyPaste(m tui.Model, text string) tui.Model {
	msg := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune(text),
		Paste: true,
	}
	updated, _ := m.Update(msg)
	return updated.(tui.Model)
}

// testModel creates a Model with a standard window size and optional onSubmit.
func testModel(onSubmit tui.SubmitFunc) tui.Model {
	m := tui.NewModel("pair-programming", []string{"coder", "reviewer"}, onSubmit)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 100, Height: 40})
	return m
}

// fillChat adds n agent messages to produce enough lines for scrolling.
func fillChat(m tui.Model, n int) tui.Model {
	for i := range n {
		m = applyMsg(m, tui.AgentOutputMsg{
			AgentName: "coder",
			Output:    fmt.Sprintf("Message %d with enough content to fill a line", i),
		})
	}
	return m
}
