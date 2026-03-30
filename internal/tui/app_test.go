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
	plain := ansiRe.ReplaceAllString(view, "")
	promptIdx := strings.Index(plain, "❯ /")
	paletteIdx := strings.Index(plain, "/help")
	require.True(t, promptIdx >= 0, "prompt should be in view")
	require.True(t, paletteIdx >= 0, "palette should be in view")
	assert.True(t, paletteIdx > promptIdx, "palette should render below the prompt, not above")
}

// --- Scroll tests ---

// fillChat adds n agent messages to produce enough lines for scrolling.
func fillChat(m tui.Model, n int) tui.Model {
	for i := 0; i < n; i++ {
		m = applyMsg(m, tui.AgentOutputMsg{
			AgentName: "coder",
			Output:    fmt.Sprintf("Message %d with enough content to fill a line", i),
		})
	}
	return m
}

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
	assert.Equal(t, 3, offset)

	m = applyKey(m, "shift+down")
	assert.Equal(t, offset-1, m.ScrollOffset())
}

func TestScrollOffset_PageUpScrollsByHalfPage(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 100)

	m = applyKey(m, "pgup")
	// Half of ~20 height minus reserved lines ≈ some positive value
	assert.Greater(t, m.ScrollOffset(), 1, "pgup should scroll by more than 1 line")
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
	plain := ansiRe.ReplaceAllString(view, "")
	assert.Contains(t, plain, "↑", "should show scroll-up indicator when scrolled up")
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
	plain := ansiRe.ReplaceAllString(view, "")
	assert.Contains(t, plain, "Line-099")

	// Scroll up a lot
	for i := 0; i < 20; i++ {
		m = applyKey(m, "pgup")
	}

	view = m.View()
	plain = ansiRe.ReplaceAllString(view, "")
	// Should see earlier content, not the latest
	assert.Contains(t, plain, "Line-0", "should show earlier messages when scrolled up")
}

// --- Mouse scroll tests ---

func TestScrollOffset_MouseWheelUpScrollsUp(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	assert.Greater(t, m.ScrollOffset(), 0, "mouse wheel up should scroll up")
}

func TestScrollOffset_MouseWheelDownScrollsDown(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	// Scroll up first
	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	offsetAfterUp := m.ScrollOffset()

	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	assert.Less(t, m.ScrollOffset(), offsetAfterUp, "mouse wheel down should scroll down")
}

func TestScrollOffset_MouseWheelDownClampedAtBottom(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 50)

	// Already at bottom
	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	assert.Equal(t, 0, m.ScrollOffset(), "mouse wheel down at bottom should stay at 0")
}

func TestScrollOffset_MouseWheelUpClampedAtTop(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = fillChat(m, 5) // few messages

	// Scroll up way past content
	for range 50 {
		m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	}

	view := m.View()
	assert.NotEmpty(t, view, "should render without panic when over-scrolled via mouse")
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

func applyMsg(m tui.Model, msg tea.Msg) tui.Model {
	updated, _ := m.Update(msg)
	return updated.(tui.Model)
}
