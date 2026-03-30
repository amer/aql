package tui_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/amer/aql/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestNewModel(t *testing.T) {
	m := tui.NewModel("pair-programming", []string{"coder", "reviewer"}, nil)
	assert.Len(t, m.Chat(), 0)
	assert.Equal(t, "", m.Input())
}

func TestModelKeyInput(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)

	m = applyKey(m, "h")
	m = applyKey(m, "i")

	assert.Equal(t, "hi", m.Input())
}

func TestModelBackspace(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)

	m = applyKey(m, "h")
	m = applyKey(m, "i")
	m = applyKey(m, "backspace")

	assert.Equal(t, "h", m.Input())
}

func TestModelSubmit(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)

	m = applyKey(m, "h")
	m = applyKey(m, "i")
	m = applyKey(m, "enter")

	assert.Equal(t, "", m.Input())
	require.Len(t, m.Chat(), 1)
	assert.Equal(t, tui.EntryUserInput, m.Chat()[0].Type)
	assert.Equal(t, "hi", m.Chat()[0].Content)
}

func TestModelSubmitCallsOnSubmit(t *testing.T) {
	var received string
	onSubmit := func(input string) tea.Cmd {
		received = input
		return nil
	}

	m := tui.NewModel("test", []string{"coder"}, onSubmit)
	m = applyKey(m, "g")
	m = applyKey(m, "o")
	m = applyKey(m, "enter")

	assert.Equal(t, "go", received)
}

func TestModelEmptySubmit(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyKey(m, "enter")
	assert.Len(t, m.Chat(), 0)
}

func TestModelStreamDelta(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)

	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "Hello "})
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "world"})

	require.Len(t, m.Chat(), 1)
	assert.Equal(t, "Hello world", m.Chat()[0].Content)
	assert.True(t, m.IsStreaming())
}

func TestModelStreamDone(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)

	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "done"})
	m = applyMsg(m, tui.AgentStreamDoneMsg{AgentName: "coder"})

	assert.False(t, m.IsStreaming())
}

func TestModelStreamError(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)

	m = applyMsg(m, tui.AgentStreamErrorMsg{
		AgentName: "coder",
		Error:     fmt.Errorf("API timeout"),
	})

	require.Len(t, m.Chat(), 1)
	assert.Equal(t, tui.EntryAgentStatus, m.Chat()[0].Type)
	assert.Equal(t, tui.AgentError, m.Chat()[0].Status)
	assert.Contains(t, m.Chat()[0].Content, "API timeout")
	assert.False(t, m.IsStreaming())
}

func TestModelBlocksInputWhileStreaming(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)

	// Start streaming
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "..."})

	// Try to submit - should be blocked
	m = applyKey(m, "x")
	m = applyKey(m, "enter")

	// Input should still be there, not submitted
	assert.Equal(t, "x", m.Input())
	// Only the stream entry, no user input entry
	assert.Len(t, m.Chat(), 1)
}

func TestModelMultipleAgentStreams(t *testing.T) {
	m := tui.NewModel("test", []string{"coder", "reviewer"}, nil)

	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "code "})
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "reviewer", Delta: "review "})
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "more"})

	require.Len(t, m.Chat(), 3)
	assert.Equal(t, "code ", m.Chat()[0].Content)
	assert.Equal(t, "review ", m.Chat()[1].Content)
	assert.Equal(t, "more", m.Chat()[2].Content)
}

func TestModelAgentOutputMsg(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)

	m = applyMsg(m, tui.AgentOutputMsg{
		AgentName: "coder",
		Output:    "Writing tests...",
	})

	require.Len(t, m.Chat(), 1)
	assert.Equal(t, tui.EntryAgentText, m.Chat()[0].Type)
	assert.Equal(t, "Writing tests...", m.Chat()[0].Content)
}

func TestModelAgentStatusMsg(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)

	m = applyMsg(m, tui.AgentStatusMsg{
		AgentName: "coder",
		Status:    tui.AgentActive,
		StatusMsg: "starting",
	})

	require.Len(t, m.Chat(), 1)
	assert.Equal(t, tui.EntryAgentStatus, m.Chat()[0].Type)
}

func TestModelAgentToolCallMsg(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)

	m = applyMsg(m, tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall:  tui.ToolCall{Name: "write_file", Content: "auth.go"},
	})

	require.Len(t, m.Chat(), 1)
	assert.Equal(t, tui.EntryAgentTool, m.Chat()[0].Type)
}

func TestModelExitCommand(t *testing.T) {
	for _, cmd := range []string{"/exit", "/quit", "/q"} {
		t.Run(cmd, func(t *testing.T) {
			m := tui.NewModel("test", []string{"coder"}, nil)
			for _, c := range cmd {
				m = applyKey(m, string(c))
			}
			_, teaCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			assert.NotNil(t, teaCmd, "should return quit command for %s", cmd)
		})
	}
}

func TestModelViewContainsPrompt(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	view := m.View()
	assert.Contains(t, view, ">")
}

func TestModelChatFlow(t *testing.T) {
	m := tui.NewModel("test", []string{"coder", "reviewer"}, nil)

	// User sends a message
	m = applyKey(m, "g")
	m = applyKey(m, "o")
	m = applyKey(m, "enter")

	// Coder streams response
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "Writing auth..."})
	m = applyMsg(m, tui.AgentStreamDoneMsg{AgentName: "coder"})

	// Reviewer responds
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "reviewer", Output: "Looks good"})

	require.Len(t, m.Chat(), 3)
	assert.Equal(t, tui.EntryUserInput, m.Chat()[0].Type)
	assert.Equal(t, tui.EntryAgentText, m.Chat()[1].Type)
	assert.Equal(t, tui.EntryAgentText, m.Chat()[2].Type)
}

func TestModelWindowResize(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	assert.NotEmpty(t, view)
}

func TestRenderChatEntryUserInput(t *testing.T) {
	entry := tui.ChatEntry{Type: tui.EntryUserInput, Content: "refactor auth"}
	result := tui.RenderChatEntry(entry, 80)
	assert.Contains(t, result, "refactor auth")
}

func TestRenderChatEntryAgentText(t *testing.T) {
	entry := tui.ChatEntry{Type: tui.EntryAgentText, AgentName: "coder", Content: "Writing code..."}
	result := tui.RenderChatEntry(entry, 80)
	plain := ansiRe.ReplaceAllString(result, "")
	assert.Contains(t, plain, "coder")
	assert.Contains(t, plain, "Writing code")
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

func TestPaletteShowsOnSlash(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyKey(m, "/")
	assert.True(t, m.IsPaletteVisible(), "palette should show when / is typed")
}

func TestPaletteHidesOnBackspaceRemovingSlash(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyKey(m, "/")
	assert.True(t, m.IsPaletteVisible())
	m = applyKey(m, "backspace")
	assert.False(t, m.IsPaletteVisible(), "palette should hide when / is removed")
}

func TestPaletteFiltersAsYouType(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyKey(m, "/")
	m = applyKey(m, "e")
	// Palette should be visible and filtered
	assert.True(t, m.IsPaletteVisible())
	filtered := m.PaletteCommands()
	for _, cmd := range filtered {
		assert.Contains(t, cmd.Name, "e")
	}
}

func TestPaletteNavigateUpDown(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyKey(m, "/")
	assert.Equal(t, 0, m.PaletteSelected())

	m = applyKey(m, "down")
	assert.Equal(t, 1, m.PaletteSelected())

	m = applyKey(m, "up")
	assert.Equal(t, 0, m.PaletteSelected())
}

func TestPaletteSelectWithTab(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyKey(m, "/")
	// Tab should autocomplete the selected command into input
	cmds := m.PaletteCommands()
	if len(cmds) > 0 {
		m = applyKey(m, "tab")
		assert.Equal(t, cmds[0].Name, m.Input())
		assert.False(t, m.IsPaletteVisible(), "palette should close after tab completion")
	}
}

func TestPaletteHidesOnEscape(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyKey(m, "/")
	assert.True(t, m.IsPaletteVisible())
	m = applyKey(m, "esc")
	assert.False(t, m.IsPaletteVisible())
	assert.Equal(t, "", m.Input(), "escape should clear input")
}

func TestPaletteClearCommand(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	// Add some chat
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "hello"})
	require.Len(t, m.Chat(), 1)

	// Type /clear and submit
	for _, c := range "/clear" {
		m = applyKey(m, string(c))
	}
	m = applyKey(m, "enter")
	assert.Len(t, m.Chat(), 0, "/clear should empty the chat")
}

func TestPaletteHelpCommand(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	for _, c := range "/help" {
		m = applyKey(m, string(c))
	}
	m = applyKey(m, "enter")
	require.Len(t, m.Chat(), 1)
	assert.Equal(t, tui.EntryAgentStatus, m.Chat()[0].Type)
	assert.Contains(t, m.Chat()[0].Content, "/exit")
}

func TestPaletteViewShowsPalette(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30})
	m = applyKey(m, "/")
	view := m.View()
	assert.Contains(t, view, "/help")
	assert.Contains(t, view, "/exit")
}

func TestPaletteRenderedBelowPrompt(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30})
	m = applyKey(m, "/")
	view := m.View()
	plain := ansiRe.ReplaceAllString(view, "")
	promptIdx := strings.Index(plain, "> /")
	paletteIdx := strings.Index(plain, "/help")
	require.True(t, promptIdx >= 0, "prompt should be in view")
	require.True(t, paletteIdx >= 0, "palette should be in view")
	assert.True(t, paletteIdx > promptIdx, "palette should render below the prompt, not above")
}

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
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEscape}
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
