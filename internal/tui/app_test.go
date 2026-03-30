package tui_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/amer/aql/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	assert.Contains(t, view, "❯")
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

	require.Len(t, m.Chat(), 4)
	assert.Equal(t, tui.EntryUserInput, m.Chat()[0].Type)
	assert.Equal(t, tui.EntryAgentText, m.Chat()[1].Type)
	assert.Equal(t, tui.EntryAgentStatus, m.Chat()[2].Type) // completion indicator
	assert.Equal(t, tui.EntryAgentText, m.Chat()[3].Type)
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
	plain := stripAnsi(result)
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
	m = applyKey(m, "h")
	m = applyKey(m, "e")
	m = applyKey(m, "l")
	// Palette should be visible and filtered — /help should be top result
	assert.True(t, m.IsPaletteVisible())
	filtered := m.PaletteCommands()
	require.True(t, len(filtered) > 0)
	assert.Equal(t, "/help", filtered[0].Name)
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
	plain := stripAnsi(view)
	promptIdx := strings.Index(plain, "❯ /")
	require.True(t, promptIdx >= 0, "prompt should be in view")
	// Find /help after the prompt (welcome tips may also contain "/help")
	paletteIdx := strings.Index(plain[promptIdx:], "/help")
	require.True(t, paletteIdx >= 0, "palette /help should appear after prompt")
	assert.True(t, paletteIdx > 0, "palette should render below the prompt, not above")
}

// --- Scroll tests ---

func TestScrollOffset_DefaultIsZero(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	assert.Equal(t, 0, m.ScrollOffset(), "default scroll offset should be 0 (at bottom)")
}

func TestScrollOffset_ShiftUpScrollsUp(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50) // many messages to exceed visible area

	m = applyKey(m, "shift+up")
	assert.Greater(t, m.ScrollOffset(), 0, "shift+up should scroll up from bottom")
}

func TestScrollOffset_ShiftDownAtBottomStaysZero(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	m = applyKey(m, "shift+down")
	assert.Equal(t, 0, m.ScrollOffset(), "shift+down at bottom should stay at 0")
}

func TestScrollOffset_ShiftUpThenDownReturns(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	m = applyKey(m, "shift+up")
	m = applyKey(m, "shift+up")
	m = applyKey(m, "shift+up")
	offset := m.ScrollOffset()
	assert.Equal(t, 9, offset) // 3 presses * 3 lines each

	m = applyKey(m, "shift+down")
	assert.Equal(t, offset-3, m.ScrollOffset()) // 1 press * 3 lines
}

func TestScrollOffset_PageUpScrollsByHalfPage(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 100)

	m = applyKey(m, "pgup")
	// Half of ~20 height minus reserved lines ≈ some positive value
	assert.GreaterOrEqual(t, m.ScrollOffset(), 1, "pgup should scroll by at least 1 line")
}

func TestScrollOffset_PageDownReducesOffset(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 100)

	m = applyKey(m, "pgup")
	m = applyKey(m, "pgup")
	after2Up := m.ScrollOffset()

	m = applyKey(m, "pgdown")
	assert.Less(t, m.ScrollOffset(), after2Up, "pgdown should reduce scroll offset")
}

func TestScrollOffset_ClampedToMax(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 5) // few messages, may not exceed visible area

	// Try scrolling up a lot — should clamp
	for i := 0; i < 50; i++ {
		m = applyKey(m, "pgup")
	}
	// Offset should not be negative or cause rendering issues
	view := m.View()
	assert.NotEmpty(t, view, "view should render without panic when over-scrolled")
}

func TestScrollOffset_AutoScrollOnNewContent(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	// At bottom (offset=0), new content should keep us at bottom
	assert.Equal(t, 0, m.ScrollOffset())
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "new message"})
	assert.Equal(t, 0, m.ScrollOffset(), "should stay at bottom when new content arrives while at bottom")
}

func TestScrollOffset_NoAutoScrollWhenScrolledUp(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	// Scroll up
	m = applyKey(m, "pgup")
	offsetBefore := m.ScrollOffset()
	assert.Greater(t, offsetBefore, 0)

	// New content arrives — should NOT auto-scroll to bottom
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "new while scrolled"})
	assert.Equal(t, offsetBefore, m.ScrollOffset(), "should preserve scroll position when new content arrives while scrolled up")
}

func TestScrollOffset_StreamingAutoScrollsWhenAtBottom(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	assert.Equal(t, 0, m.ScrollOffset())
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "streaming..."})
	assert.Equal(t, 0, m.ScrollOffset(), "streaming should keep at bottom when already at bottom")
}

func TestScrollOffset_StreamingNoAutoScrollWhenScrolledUp(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	m = applyKey(m, "pgup")
	offsetBefore := m.ScrollOffset()

	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "streaming..."})
	assert.Equal(t, offsetBefore, m.ScrollOffset(), "streaming should not auto-scroll when user scrolled up")
}

func TestStreamPhase_StartShowsUpArrow(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30})

	// AgentStreamStartMsg = requesting phase → ↑
	m = applyMsg(m, tui.AgentStreamStartMsg{AgentName: "coder"})
	view := m.View()
	plain := stripAnsi(view)
	assert.Contains(t, plain, "↑", "start (requesting) phase should show up arrow")
}

func TestStreamPhase_DeltaSwitchesToDownArrow(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30})

	m = applyMsg(m, tui.AgentStreamStartMsg{AgentName: "coder"})
	// Delta arrives = responding phase → ↓ with token count
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "hello"})
	view := m.View()
	plain := stripAnsi(view)
	assert.Contains(t, plain, "↓", "responding phase should show down arrow")
	assert.Contains(t, plain, "tokens", "responding phase should show token count")
}

func TestScrollOffset_SubmitResetsToBottom(t *testing.T) {
	submitted := false
	m := tui.NewModel("test", []string{"coder"}, func(input string) tea.Cmd {
		submitted = true
		return nil
	})
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	m = applyKey(m, "pgup")
	assert.Greater(t, m.ScrollOffset(), 0)

	// Submit resets to bottom
	for _, c := range "hello" {
		m = applyKey(m, string(c))
	}
	m = applyKey(m, "enter")
	assert.True(t, submitted)
	assert.Equal(t, 0, m.ScrollOffset(), "submitting should reset scroll to bottom")
}

func TestScrollOffset_ViewShowsIndicatorWhenScrolledUp(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	m = applyKey(m, "pgup")
	view := m.View()
	plain := stripAnsi(view)
	assert.Contains(t, plain, "↓", "should show down-arrow pointing toward hidden content below")
	assert.Contains(t, plain, "more lines below", "should indicate lines are below the viewport")
}

func TestScrollOffset_ViewShowsOlderContentWhenScrolledUp(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	// Add numbered messages
	for i := 0; i < 100; i++ {
		m = applyMsg(m, tui.AgentOutputMsg{
			AgentName: "coder",
			Output:    fmt.Sprintf("Line-%03d", i),
		})
	}

	// At bottom, should see latest
	view := m.View()
	plain := stripAnsi(view)
	assert.Contains(t, plain, "Line-099")

	// Scroll up a lot
	for i := 0; i < 20; i++ {
		m = applyKey(m, "pgup")
	}

	view = m.View()
	plain = stripAnsi(view)
	// Should see earlier content, not the latest
	assert.Contains(t, plain, "Line-0", "should show earlier messages when scrolled up")
}

// --- Mouse wheel scrolls chat history ---

func TestScrollOffset_MouseWheelScrollsBy5Lines(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	assert.Equal(t, 5, m.ScrollOffset(), "mouse wheel should scroll 5 lines per tick")
}

func TestScrollOffset_ShiftArrowScrollsBy3Lines(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	m = applyKey(m, "shift+up")
	assert.Equal(t, 3, m.ScrollOffset(), "shift+up should scroll 3 lines per press")
}

func TestScrollOffset_MouseWheelDownScrollsChat(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	// Scroll up first
	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	offsetAfterUp := m.ScrollOffset()

	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	assert.Less(t, m.ScrollOffset(), offsetAfterUp, "mouse wheel down should scroll chat down")
}

func TestScrollOffset_MouseWheelClampedAtBounds(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	// Already at bottom — wheel down stays at 0
	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	assert.Equal(t, 0, m.ScrollOffset(), "wheel down at bottom should stay at 0")
}

// --- Up/Down arrow controls history, not scroll ---

func TestUpDownArrow_ControlsHistory(t *testing.T) {
	var submitted []string
	onSubmit := func(input string) tea.Cmd {
		submitted = append(submitted, input)
		return nil
	}

	m := tui.NewModel("test", []string{"coder"}, onSubmit)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})

	// Submit two messages to build history
	m = typeString(m, "first command")
	m = applyKey(m, "enter")
	m = typeString(m, "second command")
	m = applyKey(m, "enter")

	// Up arrow should recall history, not scroll
	m = applyKey(m, "up")
	assert.Equal(t, "second command", m.Input(), "up arrow should recall previous command")
	m = applyKey(m, "up")
	assert.Equal(t, "first command", m.Input(), "up arrow again should recall earlier command")

	// Down arrow should go forward in history
	m = applyKey(m, "down")
	assert.Equal(t, "second command", m.Input(), "down arrow should go forward in history")
}

func TestUpDownArrow_DoesNotScroll(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	assert.Equal(t, 0, m.ScrollOffset())
	m = applyKey(m, "up")
	assert.Equal(t, 0, m.ScrollOffset(), "up arrow should not change scroll offset")
	m = applyKey(m, "down")
	assert.Equal(t, 0, m.ScrollOffset(), "down arrow should not change scroll offset")
}

// --- Phase 5: Transcript view integration tests ---

func TestView_TranscriptMarkers(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "Hello world"})

	view := m.View()
	stripped := stripAnsi(view)
	assert.Contains(t, stripped, "⏺", "assistant text should show transcript marker")
	assert.Contains(t, stripped, "Hello world")
}

func TestView_ToolFormattedHeader(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	m = applyMsg(m, tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall:  tui.ToolCall{Name: "read_file", Content: `{"path":"app.go"}`, Status: tui.ToolRunning, ToolID: "t1"},
	})

	view := m.View()
	stripped := stripAnsi(view)
	assert.Contains(t, stripped, "Read(app.go)", "tool should show formatted header")
	assert.NotContains(t, stripped, "read_file", "should not show raw tool name")
}

func TestView_ToolConnector(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	m = applyMsg(m, tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall:  tui.ToolCall{Name: "read_file", Content: `{"path":"app.go"}`, Status: tui.ToolRunning, ToolID: "t1"},
	})
	m = applyMsg(m, tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall:  tui.ToolCall{Name: "read_file", Content: "line1\nline2\n", Status: tui.ToolDone, ToolID: "t1"},
	})

	view := m.View()
	stripped := stripAnsi(view)
	assert.Contains(t, stripped, "⎿", "completed tool should show connector")
	assert.Contains(t, stripped, "2 lines", "should show line count summary")
}

// --- Phase 6: Ctrl+O transcript mode tests ---

func TestCtrlO_TogglesTranscriptMode(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	assert.False(t, m.IsTranscriptMode())
	m = applyKey(m, "ctrl+o")
	assert.True(t, m.IsTranscriptMode())
	m = applyKey(m, "ctrl+o")
	assert.False(t, m.IsTranscriptMode())
}

func TestCtrlO_ExpandsTools(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	m = applyMsg(m, tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall:  tui.ToolCall{Name: "read_file", Content: `{"path":"app.go"}`, Status: tui.ToolRunning, ToolID: "t1"},
	})
	m = applyMsg(m, tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall:  tui.ToolCall{Name: "read_file", Content: "package tui\nfunc main() {}\n", Status: tui.ToolDone, ToolID: "t1"},
	})

	// Normal mode: collapsed summary
	normalView := stripAnsi(m.View())
	assert.Contains(t, normalView, "2 lines")
	assert.NotContains(t, normalView, "package tui")

	// Transcript mode: expanded content
	m = applyKey(m, "ctrl+o")
	expandedView := stripAnsi(m.View())
	assert.Contains(t, expandedView, "package tui")
}

// --- Phase 7: Transcript search integration tests ---

func TestTranscriptMode_SearchNavigation(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Add some chat entries
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "Found the auth bug"})
	m = applyMsg(m, tui.AgentStreamDoneMsg{AgentName: "coder"})
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "Auth module fixed"})

	// Enter transcript mode
	m = applyKey(m, "ctrl+o")
	require.True(t, m.IsTranscriptMode())

	// Start search
	m = applyKey(m, "/")
	// Type query
	m = applyKey(m, "a")
	m = applyKey(m, "u")
	m = applyKey(m, "t")
	m = applyKey(m, "h")
	assert.Equal(t, "auth", m.TranscriptSearchQuery())

	// Confirm search
	m = applyKey(m, "enter")
	assert.True(t, len(m.TranscriptMatches()) > 0, "should find matches for 'auth'")

	// Navigate with n
	startIdx := m.TranscriptMatchIdx()
	m = applyKey(m, "n")
	if len(m.TranscriptMatches()) > 1 {
		assert.NotEqual(t, startIdx, m.TranscriptMatchIdx(), "n should advance match")
	}
}

func TestTranscriptMode_EscExits(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	m = applyKey(m, "ctrl+o")
	assert.True(t, m.IsTranscriptMode())

	m = applyKey(m, "esc")
	assert.False(t, m.IsTranscriptMode())
}

func TestTranscriptMode_SearchEscCancels(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	m = applyKey(m, "ctrl+o")
	m = applyKey(m, "/")
	m = applyKey(m, "t")
	m = applyKey(m, "e")
	m = applyKey(m, "s")
	m = applyKey(m, "t")
	assert.Equal(t, "test", m.TranscriptSearchQuery())

	// Esc should cancel search but stay in transcript mode
	m = applyKey(m, "esc")
	assert.Equal(t, "", m.TranscriptSearchQuery())
	assert.True(t, m.IsTranscriptMode(), "esc during search should not exit transcript mode")
}
