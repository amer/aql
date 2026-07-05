package tui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/amer/aql/internal/domain"
	"github.com/amer/aql/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		ToolCall: domain.ToolCall{
			Name:    "write_file",
			Content: "internal/auth/auth_test.go",
			Status:  domain.ToolDone,
		},
	})
	require.Len(t, m.Chat(), 3)
	assert.Equal(t, tui.EntryAgentTool, m.Chat()[2].Type)

	// 4. Agent finishes
	m = applyMsg(m, tui.AgentStreamDoneMsg{AgentName: "coder"})
	assert.False(t, m.IsStreaming())

	// 5. Completion indicator added
	require.Len(t, m.Chat(), 4)
	assert.Equal(t, tui.EntryAgentStatus, m.Chat()[3].Type)

	// 6. Second agent chimes in
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "reviewer", Output: "LGTM, good coverage"})
	require.Len(t, m.Chat(), 5)
	assert.Equal(t, "reviewer", m.Chat()[4].AgentName)

	// 7. User sends follow-up
	m = typeString(m, "add edge cases")
	m = applyKey(m, "enter")
	require.Len(t, m.Chat(), 6)
	assert.Equal(t, []string{"write auth tests", "add edge cases"}, submitted)

	// 7. Verify view renders without panic
	view := m.View()
	plain := stripAnsi(view)
	assert.Contains(t, plain, "Welcome back")
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

func TestIntegration_SlashClearResetsAgentContext(t *testing.T) {
	cleared := false
	m := testModel(nil)
	m.SetOnClear(func() { cleared = true })

	// Simulate some chat history
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "hello"})

	// /clear should call onClear callback
	m = typeString(m, "/clear")
	m = applyKey(m, "enter")

	assert.True(t, cleared, "/clear should call onClear to reset agent context")
	assert.Len(t, m.Chat(), 0)
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

// --- Scenario: /model opens interactive picker ---

func TestIntegration_SlashModelOpensPicker(t *testing.T) {
	m := testModel(nil)

	m = typeString(m, "/model")
	m = applyKey(m, "enter")

	assert.True(t, m.IsModelPickerVisible(), "picker should be visible after /model")

	// View should show the 3 model tiers
	view := m.View()
	plain := stripAnsi(view)
	assert.Contains(t, plain, "Sonnet")
	assert.Contains(t, plain, "Opus")
	assert.Contains(t, plain, "Haiku")
}

func TestIntegration_ModelPickerNavigateAndSelect(t *testing.T) {
	m := testModel(nil)

	// Open picker
	m = typeString(m, "/model")
	m = applyKey(m, "enter")
	require.True(t, m.IsModelPickerVisible())

	// First item selected by default — move down to Opus
	m = applyKey(m, "down")
	assert.Equal(t, 1, m.ModelPickerSelected())

	// Press enter to select Opus
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tui.Model)

	assert.False(t, m.IsModelPickerVisible(), "picker should close after selection")

	// Should emit ModelSelectedMsg with Opus model ID
	require.NotNil(t, cmd)
	msg := cmd()
	selected, ok := msg.(tui.ModelSelectedMsg)
	assert.True(t, ok, "should return ModelSelectedMsg")
	assert.Contains(t, selected.Model, "opus")

	// Chat should confirm
	require.True(t, len(m.Chat()) >= 1)
	last := m.Chat()[len(m.Chat())-1]
	assert.Contains(t, last.Content, "Opus")
}

func TestIntegration_ModelPickerEscDismisses(t *testing.T) {
	m := testModel(nil)

	m = typeString(m, "/model")
	m = applyKey(m, "enter")
	require.True(t, m.IsModelPickerVisible())

	m = applyKey(m, "esc")
	assert.False(t, m.IsModelPickerVisible(), "esc should dismiss picker")
}

func TestIntegration_ModelPickerPreSelectsCurrent(t *testing.T) {
	m := testModel(nil)
	// Set current model to Opus tier
	tiers := tui.DefaultModelTiers()
	m.SetModelName(tiers[1].ModelID) // Opus

	m = typeString(m, "/model")
	m = applyKey(m, "enter")
	require.True(t, m.IsModelPickerVisible())

	// Should pre-select Opus (index 1)
	assert.Equal(t, 1, m.ModelPickerSelected())
}

func TestIntegration_CommandPaletteFuzzyMatch(t *testing.T) {
	m := testModel(nil)

	// Type "/hlp" — fuzzy match for /help
	m = applyKey(m, "/")
	m = applyKey(m, "h")
	m = applyKey(m, "l")
	m = applyKey(m, "p")

	assert.True(t, m.IsPaletteVisible())
	cmds := m.PaletteCommands()
	require.True(t, len(cmds) > 0, "should fuzzy-match /help")
	assert.Equal(t, "/help", cmds[0].Name)
}

func TestIntegration_PaletteEnterExecutesTopMatch(t *testing.T) {
	m := testModel(nil)

	// Type "/e" — fuzzy matches /exit, /help, /clear, etc. Top should be /exit
	m = applyKey(m, "/")
	m = applyKey(m, "e")
	require.True(t, m.IsPaletteVisible())
	require.True(t, len(m.PaletteCommands()) > 0)

	// Press Enter — should execute the selected palette command, not submit "/e"
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// /exit triggers tea.Quit
	assert.NotNil(t, cmd, "should produce a quit cmd from /exit")
	assert.Len(t, m.Chat(), 0, "should not add /e as user message")
}

func TestIntegration_PaletteEnterExecutesClear(t *testing.T) {
	m := testModel(nil)

	// Add some chat history first
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "hello"})
	require.Len(t, m.Chat(), 1)

	// Type "/cl" — should fuzzy match /clear
	m = applyKey(m, "/")
	m = applyKey(m, "c")
	m = applyKey(m, "l")
	require.True(t, m.IsPaletteVisible())
	cmds := m.PaletteCommands()
	require.True(t, len(cmds) > 0)
	assert.Equal(t, "/clear", cmds[0].Name)

	// Press Enter — should execute /clear
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tui.Model)

	assert.Len(t, m.Chat(), 0, "/clear should have cleared the chat")
}

func TestIntegration_ModelPickerCustomID(t *testing.T) {
	m := testModel(nil)

	m = typeString(m, "/model")
	m = applyKey(m, "enter")
	require.True(t, m.IsModelPickerVisible())

	// Arrow down past the 3 tiers to "Use custom model ID"
	m = applyKey(m, "down") // Opus
	m = applyKey(m, "down") // Haiku
	m = applyKey(m, "down") // Custom
	assert.Equal(t, 3, m.ModelPickerSelected(), "should be on custom entry")

	// Type a custom model ID
	for _, c := range "claude-opus-4-6-20260301" {
		m = applyKey(m, string(c))
	}

	// Enter should use typed input
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tui.Model)

	assert.False(t, m.IsModelPickerVisible())
	require.NotNil(t, cmd)
	msg := cmd()
	selected, ok := msg.(tui.ModelSelectedMsg)
	assert.True(t, ok)
	assert.Equal(t, "claude-opus-4-6-20260301", selected.Model)
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

	// Type 'help' to filter — /help should be the top result
	m = applyKey(m, "h")
	m = applyKey(m, "e")
	m = applyKey(m, "l")
	m = applyKey(m, "p")
	assert.True(t, m.IsPaletteVisible())
	filtered := m.PaletteCommands()
	require.True(t, len(filtered) > 0)
	assert.Equal(t, "/help", filtered[0].Name)

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
	plain := stripAnsi(view)
	assert.Contains(t, plain, "/help")
	assert.Contains(t, plain, "/exit")
}

// --- Scenario: Palette height stability (no downward bounce) ---

func TestIntegration_PaletteHeightNeverShrinksWhileTyping(t *testing.T) {
	m := testModel(nil)

	// Type "/" — opens palette with all commands
	m = applyKey(m, "/")
	require.True(t, m.IsPaletteVisible())
	allCmds := len(m.PaletteCommands())
	require.True(t, allCmds > 3, "need enough commands to see filtering shrink the list")

	viewAll := m.View()
	linesAll := strings.Count(viewAll, "\n")

	// Type "e" — filters to fewer commands (e.g. /exit, /help with 'e')
	m = applyKey(m, "e")
	require.True(t, m.IsPaletteVisible())
	fewerCmds := len(m.PaletteCommands())
	require.True(t, fewerCmds < allCmds, "filtering should reduce command count")

	viewFewer := m.View()
	linesFewer := strings.Count(viewFewer, "\n")

	// Key assertion: view should NOT shrink (prompt must not bounce down)
	assert.GreaterOrEqual(t, linesFewer, linesAll,
		"palette filtering should not reduce total view height (prevents prompt bounce)")

	// Type more to filter even further
	m = applyKey(m, "x")
	m = applyKey(m, "i")
	require.True(t, m.IsPaletteVisible())

	viewNarrow := m.View()
	linesNarrow := strings.Count(viewNarrow, "\n")
	assert.GreaterOrEqual(t, linesNarrow, linesAll,
		"further filtering should still not reduce view height below initial palette open")
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
	m = applyMsg(m, tea.WindowSizeMsg{Width: 100, Height: 50})

	// Running tool
	m = applyMsg(m, tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall:  domain.ToolCall{Name: "bash", Content: "go test ./...", Status: domain.ToolRunning},
	})

	// Done tool
	m = applyMsg(m, tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall:  domain.ToolCall{Name: "write_file", Content: "auth.go", Status: domain.ToolDone},
	})

	// Error tool
	m = applyMsg(m, tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall:  domain.ToolCall{Name: "bash", Content: "exit 1", Status: domain.ToolError},
	})

	require.Len(t, m.Chat(), 3)
	for _, entry := range m.Chat() {
		assert.Equal(t, tui.EntryAgentTool, entry.Type)
		assert.NotNil(t, entry.ToolCall)
	}

	// Verify they render with correct status indicators
	view := m.View()
	plain := stripAnsi(view)
	assert.Contains(t, plain, "Bash")
	assert.Contains(t, plain, "Write")
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
	plain := stripAnsi(view)
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
	plain := stripAnsi(view)

	// Welcome banner
	assert.Contains(t, plain, "Welcome back")

	// Status bar elements
	assert.Contains(t, plain, "claude-sonnet-4-6")
	assert.Contains(t, plain, "tokens")

	// Prompt cursor
	assert.Contains(t, plain, "❯")
}

// --- Scenario: Streaming prompt shows spinner label ---

func TestIntegration_StreamingPromptShowsAgent(t *testing.T) {
	m := testModel(nil)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 100, Height: 50})

	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "..."})
	assert.True(t, m.IsStreaming())

	view := m.View()
	plain := stripAnsi(view)
	assert.Contains(t, plain, "Composing")
	assert.Contains(t, plain, "tokens")
}

// --- Scenario: Long conversation scrolls ---

func TestIntegration_ScrollingLongConversation(t *testing.T) {
	m := testModel(nil)

	// Fill chat with many entries
	for i := range 100 {
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
	plain := stripAnsi(view)
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

// --- Scenario: Basic paste inserts text into input ---

func TestIntegration_PasteSingleLine(t *testing.T) {
	m := testModel(nil)

	m = applyPaste(m, "hello world")
	assert.Equal(t, "hello world", m.Input(), "single-line paste should insert into input buffer")
}

// --- Scenario: Paste multi-line text inserts into input ---

func TestIntegration_PasteMultiLine(t *testing.T) {
	m := testModel(nil)

	pasted := "line one\nline two\nline three"
	m = applyPaste(m, pasted)
	assert.Contains(t, m.Input(), "line one", "multi-line paste should be present in input")
	assert.Contains(t, m.Input(), "line three", "all pasted lines should be present")
}

// --- Scenario: Pasted text can be submitted ---

func TestIntegration_PasteAndSubmit(t *testing.T) {
	var submitted string
	onSubmit := func(input string) tea.Cmd {
		submitted = input
		return nil
	}

	m := testModel(onSubmit)

	m = applyPaste(m, "pasted content")
	m = applyKey(m, "enter")

	assert.Equal(t, "pasted content", submitted, "pasted text should be submitted")
}

// --- Scenario: Paste multi-line and submit preserves newlines ---

func TestIntegration_PasteMultiLineSubmit(t *testing.T) {
	var submitted string
	onSubmit := func(input string) tea.Cmd {
		submitted = input
		return nil
	}

	m := testModel(onSubmit)

	pasted := "func main() {\n\tfmt.Println(\"hello\")\n}"
	m = applyPaste(m, pasted)
	m = applyKey(m, "enter")

	assert.Equal(t, pasted, submitted, "multi-line paste should preserve newlines on submit")
}

// --- Scenario: Paste appended to existing typed text ---

func TestIntegration_TypeThenPaste(t *testing.T) {
	m := testModel(nil)

	m = typeString(m, "review: ")
	m = applyPaste(m, "pasted code")

	assert.Equal(t, "review: pasted code", m.Input(), "paste should append to existing input")
}

// --- Scenario: Paste inserted at cursor position ---

func TestIntegration_PasteAtCursorMiddle(t *testing.T) {
	m := testModel(nil)

	m = typeString(m, "hd")
	m = applyKey(m, "left") // cursor between 'h' and 'd'
	m = applyPaste(m, "ello worl")

	assert.Equal(t, "hello world", m.Input(), "paste should insert at cursor position")
}

// --- Scenario: Empty paste is a no-op ---

func TestIntegration_PasteEmpty(t *testing.T) {
	m := testModel(nil)

	m = typeString(m, "existing")
	m = applyPaste(m, "")

	assert.Equal(t, "existing", m.Input(), "empty paste should not change input")
}

// --- Scenario: Sequential pastes accumulate ---

func TestIntegration_SequentialPastes(t *testing.T) {
	m := testModel(nil)

	m = applyPaste(m, "first ")
	m = applyPaste(m, "second")

	assert.Equal(t, "first second", m.Input(), "sequential pastes should accumulate")
}

// --- Scenario: Paste during streaming is buffered ---

func TestIntegration_PasteDuringStreaming(t *testing.T) {
	m := testModel(nil)

	// Start streaming
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "thinking..."})
	assert.True(t, m.IsStreaming())

	// Paste while streaming — should go into input buffer
	m = applyPaste(m, "buffered paste")
	assert.Equal(t, "buffered paste", m.Input(), "paste during streaming should enter input buffer")

	// Submit blocked while streaming
	m = applyKey(m, "enter")
	assert.Len(t, m.Chat(), 1, "submit should be blocked during streaming")
}

// --- Scenario: Paste with special characters ---

func TestIntegration_PasteSpecialChars(t *testing.T) {
	m := testModel(nil)

	m = applyPaste(m, "path/to/file.go:42 — error: `unexpected EOF`")
	assert.Equal(t, "path/to/file.go:42 — error: `unexpected EOF`", m.Input())
}

// --- Scenario: Ctrl+C quits during streaming ---

func TestIntegration_CtrlCQuitsDuringStreaming(t *testing.T) {
	m := testModel(nil)

	// Start streaming — model is in streaming state
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "thinking..."})
	assert.True(t, m.IsStreaming(), "should be streaming")

	// Ctrl+C should still produce a quit command
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.NotNil(t, cmd, "ctrl+c during streaming should trigger quit")
}

func TestIntegration_CtrlDQuitsDuringStreaming(t *testing.T) {
	m := testModel(nil)

	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "working..."})
	assert.True(t, m.IsStreaming())

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.NotNil(t, cmd, "ctrl+d during streaming should trigger quit")
}

func TestIntegration_EscDuringStreamingDoesNotQuit(t *testing.T) {
	m := testModel(nil)

	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "working..."})
	assert.True(t, m.IsStreaming())

	// Esc should NOT quit during streaming — it interrupts the stream instead
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)
	assert.Nil(t, cmd, "esc during streaming should not trigger quit")
	assert.False(t, m.IsStreaming(), "esc should have interrupted streaming")
}

func TestIntegration_LateDeltaAfterEscDoesNotRestartStream(t *testing.T) {
	m := testModel(nil)

	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "working..."})
	require.True(t, m.IsStreaming())

	// Esc interrupts the stream.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)
	require.False(t, m.IsStreaming())

	// A delta that was already in flight arrives after the interrupt. It must
	// not restart the stream — otherwise the spinner runs forever and input is
	// blocked until a second Esc (C3).
	updated2, cmd := m.Update(tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "late"})
	m = updated2.(tui.Model)
	assert.False(t, m.IsStreaming(), "late delta after esc must not restart the stream")
	assert.Nil(t, cmd, "late delta after esc must not emit a spinner tick")
}

func TestIntegration_CtrlCCancelsStreamContext(t *testing.T) {
	m := testModel(nil)

	// Wire up a cancel function to track if it was called
	cancelled := false
	m.SetCancelStream(func() { cancelled = true })

	// Start streaming
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "thinking..."})
	assert.True(t, m.IsStreaming())

	// Ctrl+C should cancel the stream context before quitting
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.True(t, cancelled, "ctrl+c should call cancelStream to abort the API call")
	assert.NotNil(t, cmd, "ctrl+c should still trigger quit")
}

func TestIntegration_CtrlCWithoutStreamingDoesNotCancel(t *testing.T) {
	m := testModel(nil)

	cancelled := false
	m.SetCancelStream(func() { cancelled = true })

	// Not streaming — ctrl+c should quit but not call cancel
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.False(t, cancelled, "ctrl+c when not streaming should not call cancelStream")
	assert.NotNil(t, cmd, "ctrl+c should trigger quit")
}

func TestIntegration_SlashExitDuringStreaming(t *testing.T) {
	m := testModel(nil)

	// Start streaming
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "working..."})
	assert.True(t, m.IsStreaming())

	// Type /exit and press enter — should quit even during streaming
	m = typeString(m, "/exit")
	_, cmd := applyMsgCmd(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd, "/exit during streaming should trigger quit")
}

func TestIntegration_SlashQuitDuringStreaming(t *testing.T) {
	m := testModel(nil)

	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "working..."})
	assert.True(t, m.IsStreaming())

	m = typeString(m, "/quit")
	_, cmd := applyMsgCmd(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd, "/quit during streaming should trigger quit")
}

// --- Scenario: Esc interrupts streaming ---

func TestIntegration_EscInterruptsStreaming(t *testing.T) {
	m := testModel(nil)

	// Start streaming
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "working..."})
	assert.True(t, m.IsStreaming())

	// Esc should stop streaming but NOT quit
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)

	assert.False(t, m.IsStreaming(), "esc should stop streaming")
	assert.Nil(t, cmd, "esc should not trigger quit")

	// Should have an "Interrupted" status entry in chat
	found := false
	for _, entry := range m.Chat() {
		if entry.Type == tui.EntryAgentStatus && strings.Contains(entry.Content, "Interrupted") {
			found = true
			break
		}
	}
	assert.True(t, found, "chat should contain 'Interrupted' status entry")
}

func TestIntegration_EscInterruptCancelsContext(t *testing.T) {
	m := testModel(nil)

	cancelled := false
	m.SetCancelStream(func() { cancelled = true })

	// Start streaming
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "thinking..."})
	assert.True(t, m.IsStreaming())

	// Esc should call cancelStream
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)

	assert.True(t, cancelled, "esc during streaming should call cancelStream")
	assert.False(t, m.IsStreaming())
}

func TestIntegration_EscWithoutStreamingNoOp(t *testing.T) {
	m := testModel(nil)

	// Not streaming, no palette, no picker — esc should be a no-op
	assert.False(t, m.IsStreaming())
	assert.False(t, m.IsPaletteVisible())
	assert.False(t, m.IsModelPickerVisible())

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)

	assert.Nil(t, cmd, "esc without streaming should not trigger any command")
	assert.Equal(t, 0, len(m.Chat()), "esc without streaming should not add chat entries")
}

// --- Scenario: Mouse click-drag selects text (copy-on-select) ---

func TestIntegration_MouseClickStartsSelection(t *testing.T) {
	m := testModel(nil)
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "hello world"})
	y := findLineY(m, "hello world")
	require.GreaterOrEqual(t, y, 0, "should find agent output line")

	// Left click starts selection
	m = applyMsg(m, tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      5, Y: y,
	})
	assert.True(t, m.HasSelection(), "left click should start selection")
}

func TestIntegration_MouseDragUpdatesSelection(t *testing.T) {
	m := testModel(nil)
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "hello world"})
	y := findLineY(m, "hello world")
	require.GreaterOrEqual(t, y, 0, "should find agent output line")

	// Press then drag
	m = applyMsg(m, tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      0, Y: y,
	})
	m = applyMsg(m, tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
		X:      10, Y: y,
	})
	assert.True(t, m.HasSelection(), "drag should maintain selection")
}

func TestIntegration_MouseReleaseCompletesSelection(t *testing.T) {
	m := testModel(nil)
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "hello world"})
	y := findLineY(m, "hello world")
	require.GreaterOrEqual(t, y, 0, "should find agent output line")

	// Press, drag, release
	m = applyMsg(m, tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      0, Y: y,
	})
	m = applyMsg(m, tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
		X:      5, Y: y,
	})
	// Release copies text but keeps selection visible
	m, _ = applyMsgCmd(m, tea.MouseMsg{
		Button: tea.MouseButtonNone,
		Action: tea.MouseActionRelease,
		X:      5, Y: y,
	})
	assert.True(t, m.HasSelection(), "release should keep selection visible")

	// Next click clears the old selection and starts a new one
	m = applyMsg(m, tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      0, Y: y - 1,
	})
	// Selection is active at the new position, old highlight is gone
	assert.True(t, m.HasSelection(), "new click starts new selection")
}

func TestIntegration_MouseScrollDoesNotStartSelection(t *testing.T) {
	m := testModel(nil)
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "hello"})

	m = applyMsg(m, tea.MouseMsg{
		Button: tea.MouseButtonWheelUp,
		Action: tea.MouseActionPress,
	})
	assert.False(t, m.HasSelection(), "scroll wheel should not start selection")
}

func TestIntegration_SelectionHighlightInView(t *testing.T) {
	m := testModel(nil)
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "hello world"})
	y := findLineY(m, "hello world")
	require.GreaterOrEqual(t, y, 0, "should find agent output line")

	// Start selection on the agent output line (⏺ hello world)
	m = applyMsg(m, tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      0, Y: y,
	})
	m = applyMsg(m, tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
		X:      10, Y: y,
	})

	view := m.View()
	// During active selection, View() should contain selection background color
	assert.Contains(t, view, "\x1b[48;2;", "active selection should render background highlight")
}

func TestIntegration_NoHighlightWithoutSelection(t *testing.T) {
	m := testModel(nil)
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "hello world"})

	view := m.View()
	// No selection active — no selection background
	assert.NotContains(t, view, "\x1b[48;2;46;60;100m", "no selection means no highlight background")
}

func TestIntegration_SelectionReleaseCopiesText(t *testing.T) {
	m := testModel(nil)
	m = applyMsg(m, tui.AgentOutputMsg{AgentName: "coder", Output: "hello world"})
	y := findLineY(m, "hello world")
	require.GreaterOrEqual(t, y, 0, "should find agent output line")

	// Press starts selection on the agent output line
	m = applyMsg(m, tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      0, Y: y,
	})

	// Drag
	m = applyMsg(m, tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
		X:      10, Y: y,
	})

	// Release should produce a clipboard command
	_, cmd := applyMsgCmd(m, tea.MouseMsg{
		Button: tea.MouseButtonNone,
		Action: tea.MouseActionRelease,
		X:      10, Y: y,
	})

	// The command should not be nil if text was extracted
	// (exact text depends on view layout, but we verify the cmd exists)
	assert.NotNil(t, cmd, "release with dragged selection should produce a clipboard command")
}

// --- Scenario: Paste renders in view ---

func TestIntegration_PasteVisibleInView(t *testing.T) {
	m := testModel(nil)

	m = applyPaste(m, "visible text")
	view := m.View()
	plain := stripAnsi(view)
	assert.Contains(t, plain, "visible text", "pasted text should be visible in the view")
}

// --- Scenario: Mouse wheel scrolls chat history, not prompt history ---

func TestIntegration_MouseWheelScrollsChatHistory(t *testing.T) {
	m := testModel(nil)

	// Fill chat with many entries so there's content to scroll
	for i := range 50 {
		m = applyMsg(m, tui.AgentOutputMsg{
			AgentName: "coder",
			Output:    fmt.Sprintf("Message %d", i),
		})
	}
	assert.Equal(t, 0, m.ScrollOffset(), "should start at bottom")

	// Mouse wheel up should scroll chat history up
	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	assert.Greater(t, m.ScrollOffset(), 0, "mouse wheel up should scroll chat up")

	offsetAfterUp := m.ScrollOffset()

	// Mouse wheel down should scroll chat history back down
	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	assert.Less(t, m.ScrollOffset(), offsetAfterUp, "mouse wheel down should scroll chat down")
}

func TestIntegration_MouseWheelDoesNotAffectPromptInput(t *testing.T) {
	var submitted []string
	onSubmit := func(input string) tea.Cmd {
		submitted = append(submitted, input)
		return nil
	}

	m := testModel(onSubmit)

	// Submit several prompts to build prompt history
	m = typeString(m, "first command")
	m = applyKey(m, "enter")
	m = typeString(m, "second command")
	m = applyKey(m, "enter")
	m = typeString(m, "third command")
	m = applyKey(m, "enter")

	// Input should be empty now
	assert.Equal(t, "", m.Input())

	// Mouse wheel should NOT recall prompt history — it scrolls chat only
	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	assert.Equal(t, "", m.Input(), "mouse wheel up should not change prompt input")

	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	assert.Equal(t, "", m.Input(), "mouse wheel down should not change prompt input")
}

func TestIntegration_MouseScrollClampedAtBounds(t *testing.T) {
	m := testModel(nil)

	// Fill chat
	for i := range 50 {
		m = applyMsg(m, tui.AgentOutputMsg{
			AgentName: "coder",
			Output:    fmt.Sprintf("Entry %d", i),
		})
	}

	// Scroll down at bottom should stay at 0
	m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	assert.Equal(t, 0, m.ScrollOffset(), "wheel down at bottom should clamp at 0")

	// Scroll up many times — should not panic
	for range 200 {
		m = applyMsg(m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	}

	view := m.View()
	assert.NotEmpty(t, view, "should render without panic when over-scrolled")
}

// --- Scenario: Keyboard also scrolls chat ---

func TestIntegration_ShiftArrowsScrollChat(t *testing.T) {
	m := testModel(nil)

	for i := range 50 {
		m = applyMsg(m, tui.AgentOutputMsg{
			AgentName: "coder",
			Output:    fmt.Sprintf("Message %d", i),
		})
	}
	assert.Equal(t, 0, m.ScrollOffset(), "should start at bottom")

	m = applyKey(m, "shift+up")
	assert.Greater(t, m.ScrollOffset(), 0, "shift+up should scroll chat up")

	offsetAfterUp := m.ScrollOffset()

	m = applyKey(m, "shift+down")
	assert.Less(t, m.ScrollOffset(), offsetAfterUp, "shift+down should scroll chat down")
}

func TestIntegration_PageUpDownScrollsChat(t *testing.T) {
	m := testModel(nil)

	for i := range 50 {
		m = applyMsg(m, tui.AgentOutputMsg{
			AgentName: "coder",
			Output:    fmt.Sprintf("Message %d", i),
		})
	}

	m = applyKey(m, "pgup")
	assert.Greater(t, m.ScrollOffset(), 1, "pgup should scroll chat by multiple lines")

	offsetAfterPgUp := m.ScrollOffset()

	m = applyKey(m, "pgdown")
	assert.Less(t, m.ScrollOffset(), offsetAfterPgUp, "pgdown should scroll chat down")
}

func TestIntegration_ArrowsControlPromptHistory(t *testing.T) {
	var submitted []string
	onSubmit := func(input string) tea.Cmd {
		submitted = append(submitted, input)
		return nil
	}

	m := testModel(onSubmit)

	// Submit prompts to build history
	m = typeString(m, "alpha")
	m = applyKey(m, "enter")
	m = typeString(m, "beta")
	m = applyKey(m, "enter")
	m = typeString(m, "gamma")
	m = applyKey(m, "enter")

	assert.Equal(t, "", m.Input())

	// Up arrow should recall previous prompt, not scroll chat
	scrollBefore := m.ScrollOffset()
	m = applyKey(m, "up")
	assert.Equal(t, "gamma", m.Input(), "up arrow should recall last prompt")
	assert.Equal(t, scrollBefore, m.ScrollOffset(), "up arrow should not change scroll offset")

	m = applyKey(m, "up")
	assert.Equal(t, "beta", m.Input(), "up arrow should recall earlier prompt")

	m = applyKey(m, "up")
	assert.Equal(t, "alpha", m.Input(), "up arrow should recall earliest prompt")

	// Down arrow should navigate forward through prompt history
	m = applyKey(m, "down")
	assert.Equal(t, "beta", m.Input(), "down arrow should go forward in history")

	m = applyKey(m, "down")
	assert.Equal(t, "gamma", m.Input(), "down arrow should go forward in history")
}

func TestIntegration_ArrowsDoNotScrollChat(t *testing.T) {
	m := testModel(nil)

	// Fill chat
	for i := range 50 {
		m = applyMsg(m, tui.AgentOutputMsg{
			AgentName: "coder",
			Output:    fmt.Sprintf("Line %d", i),
		})
	}

	scrollBefore := m.ScrollOffset()

	// Up/down arrows should not change scroll offset
	m = applyKey(m, "up")
	assert.Equal(t, scrollBefore, m.ScrollOffset(), "up arrow should not scroll chat")

	m = applyKey(m, "down")
	assert.Equal(t, scrollBefore, m.ScrollOffset(), "down arrow should not scroll chat")
}

// --- Scenario: ask_user pauses streaming so user can respond ---

func TestIntegration_AskUserDuringStreaming(t *testing.T) {
	var submitted []string
	onSubmit := func(input string) tea.Cmd {
		submitted = append(submitted, input)
		return nil
	}

	m := testModel(onSubmit)

	// Agent starts streaming
	m = applyMsg(m, tui.AgentStreamDeltaMsg{AgentName: "coder", Delta: "Let me ask..."})
	assert.True(t, m.IsStreaming(), "should be streaming after delta")

	// Agent issues ask_user while streaming
	responseCh := make(chan string, 1)
	m = applyMsg(m, tui.AgentAskUserMsg{
		AgentName:  "coder",
		Question:   "Which option do you prefer?",
		ResponseCh: responseCh,
	})

	// Streaming remains true — the agent is mid-turn, just waiting for input
	assert.True(t, m.IsStreaming(), "streaming state should not change")
	assert.True(t, m.HasPendingQuestion(), "should have a pending question")

	// User types and submits an answer
	m = typeString(m, "option 1")
	m = applyKey(m, "enter")

	// Answer should be sent through the channel, not as a new agent prompt
	assert.Len(t, submitted, 0, "answer should not trigger onSubmit")
	assert.False(t, m.HasPendingQuestion(), "pending question should be cleared")

	select {
	case answer := <-responseCh:
		assert.Equal(t, "option 1", answer)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected answer on response channel")
	}
}
