package tui_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/amer/aql/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var stripAnsi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string {
	return stripAnsi.ReplaceAllString(s, "")
}

// testModel creates a Model with a standard window size and optional onSubmit.
func testModel(onSubmit tui.SubmitFunc) tui.Model {
	m := tui.NewModel("pair-programming", []string{"coder", "reviewer"}, onSubmit)
	m.SetAvailableModels([]tui.ModelOption{
		{ID: "claude-haiku-4-5-20251001", DisplayName: "Claude Haiku 4.5"},
		{ID: "claude-sonnet-4-20250514", DisplayName: "Claude Sonnet 4"},
		{ID: "claude-opus-4-20250415", DisplayName: "Claude Opus 4"},
	})
	m, _ = applyMsgCmd(m, tea.WindowSizeMsg{Width: 100, Height: 40})
	return m
}

func applyMsgCmd(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(tui.Model), cmd
}

func typeString(m tui.Model, s string) tui.Model {
	for _, c := range s {
		m = applyKey(m, string(c))
	}
	return m
}

// --- Scenario: Full conversation flow ---

func TestIntegration_FullConversation(t *testing.T) {
	var submitted []string
	onSubmit := func(input string) tea.Cmd {
		submitted = append(submitted, input)
		return nil
	}

	m := testModel(onSubmit)

	// 1. User sends first message
	m = typeString(m, "write auth tests")
	m = applyKey(m, "enter")

	require.Len(t, m.Chat(), 1)
	assert.Equal(t, tui.EntryUserInput, m.Chat()[0].Type)
	assert.Equal(t, "write auth tests", m.Chat()[0].Content)
	assert.Equal(t, []string{"write auth tests"}, submitted)
	assert.Equal(t, "", m.Input())

	// 2. Agent starts streaming
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "I'll write "})
	assert.True(t, m.IsStreaming())

	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "tests for the auth module."})
	require.Len(t, m.Chat(), 2)
	assert.Equal(t, "I'll write tests for the auth module.", m.Chat()[1].Content)

	// 3. Agent uses a tool
	m = applyMsg(m, tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall: tui.ToolCall{
			Name:    "write_file",
			Content: "internal/auth/auth_test.go",
			Status:  tui.ToolDone,
		},
	})
	require.Len(t, m.Chat(), 3)
	assert.Equal(t, tui.EntryAgentTool, m.Chat()[2].Type)

	// 4. Agent finishes
	m = applyMsg(m, tui.AgentStreamDoneMsg{AgentName: "coder"})
	assert.False(t, m.IsStreaming())

	// 5. Second agent chimes in
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "reviewer", Output: "LGTM, good coverage"})
	require.Len(t, m.Chat(), 4)
	assert.Equal(t, "reviewer", m.Chat()[3].AgentName)

	// 6. User sends follow-up
	m = typeString(m, "add edge cases")
	m = applyKey(m, "enter")
	require.Len(t, m.Chat(), 5)
	assert.Equal(t, []string{"write auth tests", "add edge cases"}, submitted)

	// 7. Verify view renders without panic
	view := m.View()
	plain := strip(view)
	assert.Contains(t, plain, "AQL")
	assert.Contains(t, plain, "write auth tests")
	assert.Contains(t, plain, "add edge cases")
}

// --- Scenario: Input blocked during streaming ---

func TestIntegration_InputBlockedWhileStreaming(t *testing.T) {
	var submitted []string
	onSubmit := func(input string) tea.Cmd {
		submitted = append(submitted, input)
		return nil
	}

	m := testModel(onSubmit)

	// Start streaming
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "thinking..."})
	assert.True(t, m.IsStreaming())

	// Try to type and submit during streaming
	m = typeString(m, "interrupt")
	m = applyKey(m, "enter")

	// Should NOT have submitted
	assert.Len(t, submitted, 0)
	assert.Equal(t, "interrupt", m.Input())
	assert.Len(t, m.Chat(), 1, "only the stream entry, no user input")

	// Finish streaming
	m = applyMsg(m, tui.AgentStreamDoneMsg{AgentName: "coder"})
	assert.False(t, m.IsStreaming())

	// Now submit works
	m = applyKey(m, "enter")
	assert.Len(t, submitted, 1)
	assert.Equal(t, "interrupt", submitted[0])
}

// --- Scenario: Slash command /clear resets chat ---

func TestIntegration_SlashClear(t *testing.T) {
	m := testModel(nil)

	// Simulate some chat history
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "hello"})
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "reviewer", Output: "world"})
	require.Len(t, m.Chat(), 2)

	// /clear
	m = typeString(m, "/clear")
	m = applyKey(m, "enter")

	assert.Len(t, m.Chat(), 0, "chat should be cleared")
	assert.Equal(t, "", m.Input())
}

// --- Scenario: Slash command /help lists commands ---

func TestIntegration_SlashHelp(t *testing.T) {
	m := testModel(nil)

	m = typeString(m, "/help")
	m = applyKey(m, "enter")

	require.Len(t, m.Chat(), 1)
	content := m.Chat()[0].Content
	assert.Contains(t, content, "/exit")
	assert.Contains(t, content, "/clear")
	assert.Contains(t, content, "/agents")
	assert.Contains(t, content, "/status")
	assert.Contains(t, content, "/model")
	assert.Contains(t, content, "/help")
}

// --- Scenario: Slash command /agents shows agents ---

func TestIntegration_SlashAgents(t *testing.T) {
	m := testModel(nil)

	m = typeString(m, "/agents")
	m = applyKey(m, "enter")

	require.Len(t, m.Chat(), 1)
	assert.Contains(t, m.Chat()[0].Content, "coder")
	assert.Contains(t, m.Chat()[0].Content, "reviewer")
}

// --- Scenario: Slash command /status shows workflow ---

func TestIntegration_SlashStatus(t *testing.T) {
	m := testModel(nil)

	m = typeString(m, "/status")
	m = applyKey(m, "enter")

	require.Len(t, m.Chat(), 1)
	assert.Contains(t, m.Chat()[0].Content, "pair-programming")
	assert.Contains(t, m.Chat()[0].Content, "coder")
}

// --- Scenario: Slash command /model lists available models ---

func TestIntegration_SlashModelShowsList(t *testing.T) {
	m := testModel(nil)

	m = typeString(m, "/model")
	m = applyKey(m, "enter")

	require.Len(t, m.Chat(), 1)
	content := m.Chat()[0].Content
	assert.Contains(t, content, "Claude Haiku 4.5")
	assert.Contains(t, content, "Claude Sonnet 4")
	assert.Contains(t, content, "Claude Opus 4")
}

func TestIntegration_SlashModelSelect(t *testing.T) {
	m := testModel(nil)

	m = typeString(m, "/model sonnet")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tui.Model)

	// Should emit a ModelSelectedMsg with the full model ID
	require.NotNil(t, cmd)
	msg := cmd()
	selected, ok := msg.(tui.ModelSelectedMsg)
	assert.True(t, ok, "should return ModelSelectedMsg")
	assert.Equal(t, "claude-sonnet-4-20250514", selected.Model)

	// Chat should confirm the selection
	require.True(t, len(m.Chat()) >= 1)
	last := m.Chat()[len(m.Chat())-1]
	assert.Contains(t, last.Content, "Claude Sonnet 4")
}

// --- Scenario: Exit commands trigger quit ---

func TestIntegration_ExitCommands(t *testing.T) {
	for _, cmd := range []string{"/exit", "/quit", "/q"} {
		t.Run(cmd, func(t *testing.T) {
			m := testModel(nil)
			m = typeString(m, cmd)
			_, teaCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			// tea.Quit returns a non-nil Cmd
			assert.NotNil(t, teaCmd, "%s should trigger quit", cmd)
		})
	}
}

// --- Scenario: Ctrl+C always quits ---

func TestIntegration_CtrlCQuits(t *testing.T) {
	m := testModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.NotNil(t, cmd, "ctrl+c should trigger quit")
}

// --- Scenario: Multiline input ---

func TestIntegration_MultilineInput(t *testing.T) {
	var submitted []string
	onSubmit := func(input string) tea.Cmd {
		submitted = append(submitted, input)
		return nil
	}

	m := testModel(onSubmit)

	// Type first line
	m = typeString(m, "line one")

	// Alt+Enter for newline
	m, _ = applyMsgCmd(m, tea.KeyMsg{Type: tea.KeyEnter, Alt: true})

	// Type second line
	m = typeString(m, "line two")

	// Submit
	m = applyKey(m, "enter")

	require.Len(t, submitted, 1)
	assert.Equal(t, "line one\nline two", submitted[0])
	require.Len(t, m.Chat(), 1)
	assert.Equal(t, "line one\nline two", m.Chat()[0].Content)
}

// --- Scenario: Command palette interaction ---

func TestIntegration_CommandPalette(t *testing.T) {
	m := testModel(nil)

	// Type / to open palette
	m = applyKey(m, "/")
	assert.True(t, m.IsPaletteVisible())
	cmds := m.PaletteCommands()
	assert.True(t, len(cmds) > 0)

	// Type 'h' to filter
	m = applyKey(m, "h")
	assert.True(t, m.IsPaletteVisible())
	filtered := m.PaletteCommands()
	for _, cmd := range filtered {
		assert.Contains(t, cmd.Name, "h")
	}

	// Navigate down
	m = applyKey(m, "down")
	sel := m.PaletteSelected()
	if len(filtered) > 1 {
		assert.Equal(t, 1, sel)
	}

	// Tab to autocomplete
	m = applyKey(m, "up") // go back to 0
	cmdsNow := m.PaletteCommands()
	m = applyKey(m, "tab")
	assert.False(t, m.IsPaletteVisible())
	if len(cmdsNow) > 0 {
		assert.Equal(t, cmdsNow[0].Name, m.Input())
	}

	// Escape clears
	m = applyKey(m, "/") // reopen
	m, _ = applyMsgCmd(m, tea.KeyMsg{Type: tea.KeyEscape})
	assert.False(t, m.IsPaletteVisible())
	assert.Equal(t, "", m.Input())
}

// --- Scenario: Palette visible in view ---

func TestIntegration_PaletteRendersInView(t *testing.T) {
	m := testModel(nil)
	m = applyKey(m, "/")

	view := m.View()
	plain := strip(view)
	assert.Contains(t, plain, "/help")
	assert.Contains(t, plain, "/exit")
}

// --- Scenario: Stream error displays in chat ---

func TestIntegration_StreamError(t *testing.T) {
	m := testModel(nil)

	// Send error
	m = applyMsg(m, tui.AgentStreamErrorMsg{
		AgentName: "coder",
		Error:     fmt.Errorf("API rate limit exceeded"),
	})

	require.Len(t, m.Chat(), 1)
	assert.Equal(t, tui.EntryAgentStatus, m.Chat()[0].Type)
	assert.Equal(t, tui.AgentError, m.Chat()[0].Status)
	assert.Contains(t, m.Chat()[0].Content, "rate limit")
	assert.False(t, m.IsStreaming())
}

// --- Scenario: Agent status changes ---

func TestIntegration_AgentStatusUpdates(t *testing.T) {
	m := testModel(nil)

	// Agent waiting
	m = applyMsg(m, tui.AgentStatusMsg{
		AgentName: "reviewer",
		Status:    tui.AgentWaiting,
		StatusMsg: "waiting for code_written event",
	})

	// Agent active
	m = applyMsg(m, tui.AgentStatusMsg{
		AgentName: "reviewer",
		Status:    tui.AgentActive,
		StatusMsg: "reviewing code",
	})

	// Agent done
	m = applyMsg(m, tui.AgentStatusMsg{
		AgentName: "reviewer",
		Status:    tui.AgentDone,
		StatusMsg: "review complete",
	})

	require.Len(t, m.Chat(), 3)
	assert.Equal(t, tui.AgentWaiting, m.Chat()[0].Status)
	assert.Equal(t, tui.AgentActive, m.Chat()[1].Status)
	assert.Equal(t, tui.AgentDone, m.Chat()[2].Status)
}

// --- Scenario: Tool calls with different statuses ---

func TestIntegration_ToolCallStatuses(t *testing.T) {
	m := testModel(nil)

	// Running tool
	m = applyMsg(m, tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall:  tui.ToolCall{Name: "bash", Content: "go test ./...", Status: tui.ToolRunning},
	})

	// Done tool
	m = applyMsg(m, tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall:  tui.ToolCall{Name: "write_file", Content: "auth.go", Status: tui.ToolDone},
	})

	// Error tool
	m = applyMsg(m, tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall:  tui.ToolCall{Name: "bash", Content: "exit 1", Status: tui.ToolError},
	})

	require.Len(t, m.Chat(), 3)
	for _, entry := range m.Chat() {
		assert.Equal(t, tui.EntryAgentTool, entry.Type)
		assert.NotNil(t, entry.ToolCall)
	}

	// Verify they render with correct status indicators
	view := m.View()
	plain := strip(view)
	assert.Contains(t, plain, "bash")
	assert.Contains(t, plain, "write_file")
}

// --- Scenario: Multiple agents streaming interleaved ---

func TestIntegration_InterleavedMultiAgentStreaming(t *testing.T) {
	m := testModel(nil)

	// Coder starts
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "Writing "})
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "auth module"})

	// Reviewer chimes in
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "reviewer", Delta: "Reviewing approach"})

	// Coder continues (new entry since reviewer interrupted)
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "...done"})

	require.Len(t, m.Chat(), 3)
	assert.Equal(t, "Writing auth module", m.Chat()[0].Content)
	assert.Equal(t, "coder", m.Chat()[0].AgentName)
	assert.Equal(t, "Reviewing approach", m.Chat()[1].Content)
	assert.Equal(t, "reviewer", m.Chat()[1].AgentName)
	assert.Equal(t, "...done", m.Chat()[2].Content)
	assert.Equal(t, "coder", m.Chat()[2].AgentName)
}

// --- Scenario: Markdown renders in agent output ---

func TestIntegration_MarkdownInAgentOutput(t *testing.T) {
	m := testModel(nil)

	m = applyMsg(m, tui.AgentOutputMsg{
		AgentName: "coder",
		Output:    "Here is the fix:\n```go\nfunc Fix() {}\n```\nDone.",
	})

	require.Len(t, m.Chat(), 1)
	view := m.View()
	plain := strip(view)
	assert.Contains(t, plain, "func Fix()")
}

// --- Scenario: Window resize ---

func TestIntegration_WindowResize(t *testing.T) {
	m := testModel(nil)

	// Add content
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "hello"})

	// Resize small
	m, _ = applyMsgCmd(m, tea.WindowSizeMsg{Width: 40, Height: 10})
	view1 := m.View()
	assert.NotEmpty(t, view1)

	// Resize large
	m, _ = applyMsgCmd(m, tea.WindowSizeMsg{Width: 200, Height: 60})
	view2 := m.View()
	assert.NotEmpty(t, view2)

	// Both should render without panic
	assert.NotEqual(t, view1, view2)
}

// --- Scenario: Empty submit does nothing ---

func TestIntegration_EmptySubmitIgnored(t *testing.T) {
	var submitted []string
	onSubmit := func(input string) tea.Cmd {
		submitted = append(submitted, input)
		return nil
	}

	m := testModel(onSubmit)
	m = applyKey(m, "enter")
	m = applyKey(m, "enter")
	m = applyKey(m, "enter")

	assert.Len(t, submitted, 0)
	assert.Len(t, m.Chat(), 0)
}

// --- Scenario: Backspace on empty input is safe ---

func TestIntegration_BackspaceOnEmpty(t *testing.T) {
	m := testModel(nil)
	m = applyKey(m, "backspace")
	m = applyKey(m, "backspace")
	assert.Equal(t, "", m.Input())
}

// --- Scenario: Header and status bar appear in view ---

func TestIntegration_ViewLayout(t *testing.T) {
	m := testModel(nil)

	view := m.View()
	plain := strip(view)

	// Header elements
	assert.Contains(t, plain, "AQL")

	// Status bar elements
	assert.Contains(t, plain, "agents on")
	assert.Contains(t, plain, "auto-compact")

	// Prompt cursor
	assert.Contains(t, plain, "❯")
}

// --- Scenario: Streaming prompt shows spinner label ---

func TestIntegration_StreamingPromptShowsAgent(t *testing.T) {
	m := testModel(nil)

	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "..."})
	assert.True(t, m.IsStreaming())

	view := m.View()
	plain := strip(view)
	assert.Contains(t, plain, "coder")
	assert.Contains(t, plain, "responding")
}

// --- Scenario: Long conversation scrolls ---

func TestIntegration_ScrollingLongConversation(t *testing.T) {
	m := testModel(nil)

	// Fill chat with many entries
	for i := 0; i < 100; i++ {
		m = applyMsg(m, tui.AgentOutputMsg{
			AgentName: "coder",
			Output:    fmt.Sprintf("Message %d with some content", i),
		})
	}

	require.Len(t, m.Chat(), 100)

	// View should render without panic
	view := m.View()
	assert.NotEmpty(t, view)

	// Latest messages should be visible (auto-scroll)
	plain := strip(view)
	assert.Contains(t, plain, "Message 99")
}

// --- Scenario: /clear then new conversation ---

func TestIntegration_ClearThenContinue(t *testing.T) {
	var submitted []string
	onSubmit := func(input string) tea.Cmd {
		submitted = append(submitted, input)
		return nil
	}

	m := testModel(onSubmit)

	// First conversation
	m = typeString(m, "hello")
	m = applyKey(m, "enter")
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "hi there"})
	require.Len(t, m.Chat(), 2)

	// Clear
	m = typeString(m, "/clear")
	m = applyKey(m, "enter")
	assert.Len(t, m.Chat(), 0)

	// New conversation
	m = typeString(m, "new topic")
	m = applyKey(m, "enter")
	require.Len(t, m.Chat(), 1)
	assert.Equal(t, "new topic", m.Chat()[0].Content)
	assert.Equal(t, []string{"hello", "new topic"}, submitted)
}
