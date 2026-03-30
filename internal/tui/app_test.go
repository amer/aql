package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewModel(t *testing.T) {
	m := tui.NewModel("pair-programming", []string{"coder", "reviewer"})
	panels := m.AgentPanels()

	assert.Len(t, panels, 2)
	assert.Equal(t, "coder", panels[0].Name)
	assert.Equal(t, tui.AgentWaiting, panels[0].Status)
}

func TestModelViewContainsAllParts(t *testing.T) {
	m := tui.NewModel("test-wf", []string{"coder", "reviewer"})
	view := m.View()

	assert.Contains(t, view, "AQL")
	assert.Contains(t, view, "test-wf")
	assert.Contains(t, view, "coder")
	assert.Contains(t, view, "reviewer")
}

func TestModelKeyInput(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})

	m, _ = applyKey(m, "h")
	m, _ = applyKey(m, "i")

	assert.Equal(t, "hi", m.Input())
}

func TestModelBackspace(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})

	m, _ = applyKey(m, "h")
	m, _ = applyKey(m, "i")
	m, _ = applyKey(m, "backspace")

	assert.Equal(t, "h", m.Input())
}

func TestModelSubmit(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})

	m, _ = applyKey(m, "h")
	m, _ = applyKey(m, "i")
	m, _ = applyKey(m, "enter")

	assert.Equal(t, "", m.Input())
	assert.Equal(t, []string{"hi"}, m.Submitted())
}

func TestModelEmptySubmit(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})

	m, _ = applyKey(m, "enter")

	assert.Len(t, m.Submitted(), 0)
}

func TestModelAgentOutputMsg(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})

	updated, _ := m.Update(tui.AgentOutputMsg{
		AgentName: "coder",
		Output:    "Writing tests...",
	})
	m = updated.(tui.Model)

	assert.Equal(t, "Writing tests...", m.AgentPanels()[0].Output)
}

func TestModelAgentStatusMsg(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})

	updated, _ := m.Update(tui.AgentStatusMsg{
		AgentName: "coder",
		Status:    tui.AgentActive,
		StatusMsg: "running",
	})
	m = updated.(tui.Model)

	assert.Equal(t, tui.AgentActive, m.AgentPanels()[0].Status)
	assert.Equal(t, "running", m.AgentPanels()[0].StatusMsg)
}

func TestModelAgentToolCallMsg(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})

	updated, _ := m.Update(tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall:  tui.ToolCall{Name: "write_file", Content: "auth.go"},
	})
	m = updated.(tui.Model)

	require.Len(t, m.AgentPanels()[0].ToolCalls, 1)
	assert.Equal(t, "write_file", m.AgentPanels()[0].ToolCalls[0].Name)
}

func TestModelWindowResize(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)

	view := m.View()
	assert.NotEmpty(t, view)
}

func TestModelIgnoresUnknownAgent(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})

	updated, _ := m.Update(tui.AgentOutputMsg{
		AgentName: "nonexistent",
		Output:    "ignored",
	})
	m = updated.(tui.Model)

	assert.Equal(t, "", m.AgentPanels()[0].Output)
}

func applyKey(m tui.Model, key string) (tui.Model, tea.Cmd) {
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	if key == "enter" {
		updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}
	if key == "backspace" {
		updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	return updated.(tui.Model), cmd
}
