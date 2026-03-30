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
	assert.Len(t, m.Chat(), 0)
	assert.Equal(t, "", m.Input())
}

func TestModelKeyInput(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})

	m = applyKey(m, "h")
	m = applyKey(m, "i")

	assert.Equal(t, "hi", m.Input())
}

func TestModelBackspace(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})

	m = applyKey(m, "h")
	m = applyKey(m, "i")
	m = applyKey(m, "backspace")

	assert.Equal(t, "h", m.Input())
}

func TestModelSubmit(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})

	m = applyKey(m, "h")
	m = applyKey(m, "i")
	m = applyKey(m, "enter")

	assert.Equal(t, "", m.Input())
	require.Len(t, m.Chat(), 1)
	assert.Equal(t, tui.EntryUserInput, m.Chat()[0].Type)
	assert.Equal(t, "hi", m.Chat()[0].Content)
}

func TestModelEmptySubmit(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})
	m = applyKey(m, "enter")
	assert.Len(t, m.Chat(), 0)
}

func TestModelAgentOutputMsg(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})

	m = applyMsg(m, tui.AgentOutputMsg{
		AgentName: "coder",
		Output:    "Writing tests...",
	})

	require.Len(t, m.Chat(), 1)
	assert.Equal(t, tui.EntryAgentText, m.Chat()[0].Type)
	assert.Equal(t, "coder", m.Chat()[0].AgentName)
	assert.Equal(t, "Writing tests...", m.Chat()[0].Content)
}

func TestModelAgentStatusMsg(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})

	m = applyMsg(m, tui.AgentStatusMsg{
		AgentName: "coder",
		Status:    tui.AgentActive,
		StatusMsg: "starting",
	})

	require.Len(t, m.Chat(), 1)
	assert.Equal(t, tui.EntryAgentStatus, m.Chat()[0].Type)
	assert.Equal(t, tui.AgentActive, m.Chat()[0].Status)
}

func TestModelAgentToolCallMsg(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})

	m = applyMsg(m, tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall:  tui.ToolCall{Name: "write_file", Content: "auth.go"},
	})

	require.Len(t, m.Chat(), 1)
	assert.Equal(t, tui.EntryAgentTool, m.Chat()[0].Type)
	assert.Equal(t, "write_file", m.Chat()[0].ToolCall.Name)
}

func TestModelViewContainsPrompt(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})
	view := m.View()
	assert.Contains(t, view, ">")
}

func TestModelChatFlow(t *testing.T) {
	m := tui.NewModel("test", []string{"coder", "reviewer"})

	// User sends a message
	m = applyKey(m, "g")
	m = applyKey(m, "o")
	m = applyKey(m, "enter")

	// Coder responds
	m = applyMsg(m, tui.AgentStatusMsg{AgentName: "coder", Status: tui.AgentActive, StatusMsg: "working"})
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "Writing auth module..."})
	m = applyMsg(m, tui.AgentToolCallMsg{AgentName: "coder", ToolCall: tui.ToolCall{Name: "write_file", Content: "auth.go"}})

	// Reviewer responds
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "reviewer", Output: "Looks good, consider adding error handling"})

	require.Len(t, m.Chat(), 5)

	// Verify the order
	assert.Equal(t, tui.EntryUserInput, m.Chat()[0].Type)
	assert.Equal(t, tui.EntryAgentStatus, m.Chat()[1].Type)
	assert.Equal(t, tui.EntryAgentText, m.Chat()[2].Type)
	assert.Equal(t, tui.EntryAgentTool, m.Chat()[3].Type)
	assert.Equal(t, tui.EntryAgentText, m.Chat()[4].Type)
	assert.Equal(t, "reviewer", m.Chat()[4].AgentName)
}

func TestRenderChatEntryUserInput(t *testing.T) {
	entry := tui.ChatEntry{Type: tui.EntryUserInput, Content: "refactor auth"}
	result := tui.RenderChatEntry(entry, 80)
	assert.Contains(t, result, "refactor auth")
}

func TestRenderChatEntryAgentText(t *testing.T) {
	entry := tui.ChatEntry{Type: tui.EntryAgentText, AgentName: "coder", Content: "Writing code..."}
	result := tui.RenderChatEntry(entry, 80)
	assert.Contains(t, result, "coder")
	assert.Contains(t, result, "Writing code")
}

func TestRenderChatEntryTool(t *testing.T) {
	entry := tui.ChatEntry{
		Type:      tui.EntryAgentTool,
		AgentName: "coder",
		ToolCall:  &tui.ToolCall{Name: "bash", Content: "go test ./..."},
	}
	result := tui.RenderChatEntry(entry, 80)
	assert.Contains(t, result, "bash")
	assert.Contains(t, result, "go test")
}

func TestRenderChatEntryStatus(t *testing.T) {
	entry := tui.ChatEntry{
		Type:      tui.EntryAgentStatus,
		AgentName: "reviewer",
		Status:    tui.AgentWaiting,
		Content:   "waiting for code",
	}
	result := tui.RenderChatEntry(entry, 80)
	assert.Contains(t, result, "reviewer")
	assert.Contains(t, result, "waiting for code")
}

func TestModelExitCommand(t *testing.T) {
	for _, cmd := range []string{"/exit", "/quit", "/q"} {
		t.Run(cmd, func(t *testing.T) {
			m := tui.NewModel("test", []string{"coder"})
			for _, c := range cmd {
				m = applyKey(m, string(c))
			}
			_, teaCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			assert.NotNil(t, teaCmd, "should return quit command for %s", cmd)
		})
	}
}

func TestModelWindowResize(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"})
	m = applyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	assert.NotEmpty(t, view)
}

func applyKey(m tui.Model, key string) tui.Model {
	var msg tea.Msg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "backspace":
		msg = tea.KeyMsg{Type: tea.KeyBackspace}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, _ := m.Update(msg)
	return updated.(tui.Model)
}

func applyMsg(m tui.Model, msg tea.Msg) tui.Model {
	updated, _ := m.Update(msg)
	return updated.(tui.Model)
}
