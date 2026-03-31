package tui_test

import (
	"strings"
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolDisplayName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"read_file", "Read"},
		{"write_file", "Write"},
		{"edit", "Update"},
		{"list_directory", "List"},
		{"bash", "Bash"},
		{"grep", "Grep"},
		{"glob", "Glob"},
		{"web_fetch", "Fetch"},
		{"web_search", "Search"},
		{"ask_user", "Ask"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, tui.ToolDisplayName(tt.input))
		})
	}
}

func TestToolDisplayName_Unknown(t *testing.T) {
	assert.Equal(t, "custom_tool", tui.ToolDisplayName("custom_tool"))
}

func TestFormatToolHeader(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		input    string
		expected string
	}{
		{"read_file", "read_file", `{"path":"internal/tui/app.go"}`, "Read(internal/tui/app.go)"},
		{"write_file", "write_file", `{"path":"internal/tui/new.go","content":"package tui"}`, "Write(internal/tui/new.go)"},
		{"edit", "edit", `{"path":"app.go","old_string":"foo","new_string":"bar"}`, "Update(app.go)"},
		{"bash", "bash", `{"command":"go test ./..."}`, "Bash(go test ./...)"},
		{"grep", "grep", `{"pattern":"ToolCall","path":"internal/"}`, `Grep("ToolCall", internal/)`},
		{"glob", "glob", `{"pattern":"**/*.go"}`, "Glob(**/*.go)"},
		{"web_fetch", "web_fetch", `{"url":"https://example.com/api"}`, "Fetch(https://example.com/api)"},
		{"web_search", "web_search", `{"query":"bubbletea scrolling"}`, `Search("bubbletea scrolling")`},
		{"ask_user", "ask_user", `{"question":"What model?"}`, `Ask("What model?")`},
		{"list_directory", "list_directory", `{"path":"internal/tui"}`, "List(internal/tui)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tui.FormatToolHeader(tt.tool, tt.input))
		})
	}
}

func TestFormatToolHeader_InvalidJSON(t *testing.T) {
	result := tui.FormatToolHeader("read_file", "not json")
	assert.Equal(t, "Read", result)
}

func TestFormatToolHeader_LongInputTruncation(t *testing.T) {
	longPath := "internal/" + strings.Repeat("a", 200) + ".go"
	input := `{"path":"` + longPath + `"}`
	result := tui.FormatToolHeader("read_file", input)
	assert.LessOrEqual(t, len(result), 100)
	assert.Contains(t, result, "...")
}

func TestFormatToolSummary(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		output   string
		isError  bool
		contains string
	}{
		{"read_file lines", "read_file", "line1\nline2\nline3\n", false, "3 lines"},
		{"read_file single", "read_file", "single line", false, "1 line"},
		{"bash output", "bash", "ok  github.com/amer/aql  0.045s\nmore output", false, "ok  github.com/amer/aql  0.045s"},
		{"empty output", "bash", "", false, "(no output)"},
		{"error output", "bash", "exit status 1", true, "exit status 1"},
		{"list_directory", "list_directory", "file1.go\nfile2.go\nfile3.go\n", false, "3 items"},
		{"write_file", "write_file", "line1\nline2\nline3\nline4\n", false, "4 lines"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tui.FormatToolSummary(tt.tool, tt.output, tt.isError)
			assert.Contains(t, result, tt.contains)
		})
	}
}

// --- Phase 3: TranscriptBlock grouping tests ---

func TestBuildBlocks_UserEntry(t *testing.T) {
	entries := []tui.ChatEntry{
		{Type: tui.EntryUserInput, Content: "hello"},
	}
	blocks := tui.BuildTranscriptBlocks(entries)
	require.Len(t, blocks, 1)
	assert.Equal(t, tui.BlockUser, blocks[0].Type)
	assert.Equal(t, []string{"hello"}, blocks[0].TextParts)
}

func TestBuildBlocks_TextAndTool(t *testing.T) {
	entries := []tui.ChatEntry{
		{Type: tui.EntryAgentText, AgentName: "coder", Content: "Let me read that file."},
		{Type: tui.EntryAgentTool, AgentName: "coder", ToolCall: &tui.ToolCall{
			Name: "read_file", ToolID: "t1", Content: `{"path":"app.go"}`, Status: tui.ToolRunning,
		}},
	}
	blocks := tui.BuildTranscriptBlocks(entries)
	require.Len(t, blocks, 1)
	assert.Equal(t, tui.BlockAssistant, blocks[0].Type)
	assert.Equal(t, "coder", blocks[0].AgentName)
	assert.Equal(t, []string{"Let me read that file."}, blocks[0].TextParts)
	require.Len(t, blocks[0].Tools, 1)
	assert.Equal(t, "read_file", blocks[0].Tools[0].Call.Name)
	assert.Nil(t, blocks[0].Tools[0].Result)
}

func TestBuildBlocks_TwoPhaseToolMerge(t *testing.T) {
	entries := []tui.ChatEntry{
		{Type: tui.EntryAgentTool, AgentName: "coder", ToolCall: &tui.ToolCall{
			Name: "read_file", ToolID: "t1", Content: `{"path":"app.go"}`, Status: tui.ToolRunning,
		}},
		{Type: tui.EntryAgentTool, AgentName: "coder", ToolCall: &tui.ToolCall{
			Name: "read_file", ToolID: "t1", Content: "file contents here", Status: tui.ToolDone,
		}},
	}
	blocks := tui.BuildTranscriptBlocks(entries)
	require.Len(t, blocks, 1)
	require.Len(t, blocks[0].Tools, 1)
	assert.Equal(t, tui.ToolRunning, blocks[0].Tools[0].Call.Status)
	require.NotNil(t, blocks[0].Tools[0].Result)
	assert.Equal(t, tui.ToolDone, blocks[0].Tools[0].Result.Status)
	assert.Equal(t, "file contents here", blocks[0].Tools[0].Result.Content)
}

func TestBuildBlocks_DifferentAgentsSplit(t *testing.T) {
	entries := []tui.ChatEntry{
		{Type: tui.EntryAgentText, AgentName: "coder", Content: "text from coder"},
		{Type: tui.EntryAgentText, AgentName: "reviewer", Content: "text from reviewer"},
	}
	blocks := tui.BuildTranscriptBlocks(entries)
	require.Len(t, blocks, 2)
	assert.Equal(t, "coder", blocks[0].AgentName)
	assert.Equal(t, "reviewer", blocks[1].AgentName)
}

func TestBuildBlocks_CompletionAttaches(t *testing.T) {
	entries := []tui.ChatEntry{
		{Type: tui.EntryAgentText, AgentName: "coder", Content: "done"},
		{Type: tui.EntryAgentStatus, AgentName: "coder", Content: "✻ Crunched for 2s", Status: tui.AgentDone},
	}
	blocks := tui.BuildTranscriptBlocks(entries)
	require.Len(t, blocks, 1)
	assert.Equal(t, tui.BlockAssistant, blocks[0].Type)
	assert.Equal(t, "✻ Crunched for 2s", blocks[0].StatusText)
}

func TestBuildBlocks_StandaloneStatus(t *testing.T) {
	entries := []tui.ChatEntry{
		{Type: tui.EntryAgentStatus, AgentName: "coder", Content: "What model?", Status: tui.AgentWaiting},
	}
	blocks := tui.BuildTranscriptBlocks(entries)
	require.Len(t, blocks, 1)
	assert.Equal(t, tui.BlockStatus, blocks[0].Type)
	assert.Equal(t, "What model?", blocks[0].StatusText)
}

func TestBuildBlocks_EmptyInput(t *testing.T) {
	blocks := tui.BuildTranscriptBlocks(nil)
	assert.Empty(t, blocks)
}

func TestBuildBlocks_ComplexConversation(t *testing.T) {
	entries := []tui.ChatEntry{
		{Type: tui.EntryUserInput, Content: "read the file"},
		{Type: tui.EntryAgentText, AgentName: "coder", Content: "Let me read it."},
		{Type: tui.EntryAgentTool, AgentName: "coder", ToolCall: &tui.ToolCall{
			Name: "read_file", ToolID: "t1", Content: `{"path":"app.go"}`, Status: tui.ToolRunning,
		}},
		{Type: tui.EntryAgentTool, AgentName: "coder", ToolCall: &tui.ToolCall{
			Name: "read_file", ToolID: "t1", Content: "contents", Status: tui.ToolDone,
		}},
		{Type: tui.EntryAgentText, AgentName: "coder", Content: "Here's what I found."},
		{Type: tui.EntryAgentStatus, AgentName: "coder", Content: "✻ Done", Status: tui.AgentDone},
		{Type: tui.EntryUserInput, Content: "thanks"},
	}
	blocks := tui.BuildTranscriptBlocks(entries)
	require.Len(t, blocks, 3) // user, assistant, user
	assert.Equal(t, tui.BlockUser, blocks[0].Type)
	assert.Equal(t, tui.BlockAssistant, blocks[1].Type)
	assert.Equal(t, tui.BlockUser, blocks[2].Type)

	// Assistant block should have 2 text parts, 1 tool, and status
	assert.Len(t, blocks[1].TextParts, 2)
	assert.Len(t, blocks[1].Tools, 1)
	assert.NotNil(t, blocks[1].Tools[0].Result)
	assert.Equal(t, "✻ Done", blocks[1].StatusText)
}

func TestGroupTools_Single(t *testing.T) {
	tools := []tui.ToolEntry{
		{Call: tui.ToolCall{Name: "read_file"}},
	}
	groups := tui.GroupConsecutiveTools(tools)
	require.Len(t, groups, 1)
	assert.Equal(t, "Read", groups[0].DisplayName)
	assert.Len(t, groups[0].Entries, 1)
}

func TestGroupTools_ConsecutiveReads(t *testing.T) {
	tools := []tui.ToolEntry{
		{Call: tui.ToolCall{Name: "read_file"}},
		{Call: tui.ToolCall{Name: "read_file"}},
		{Call: tui.ToolCall{Name: "read_file"}},
		{Call: tui.ToolCall{Name: "read_file"}},
	}
	groups := tui.GroupConsecutiveTools(tools)
	require.Len(t, groups, 1)
	assert.Equal(t, "Read", groups[0].DisplayName)
	assert.Len(t, groups[0].Entries, 4)
}

func TestGroupTools_Mixed(t *testing.T) {
	tools := []tui.ToolEntry{
		{Call: tui.ToolCall{Name: "read_file"}},
		{Call: tui.ToolCall{Name: "read_file"}},
		{Call: tui.ToolCall{Name: "bash"}},
		{Call: tui.ToolCall{Name: "read_file"}},
	}
	groups := tui.GroupConsecutiveTools(tools)
	require.Len(t, groups, 3)
	assert.Equal(t, "Read", groups[0].DisplayName)
	assert.Len(t, groups[0].Entries, 2)
	assert.Equal(t, "Bash", groups[1].DisplayName)
	assert.Len(t, groups[1].Entries, 1)
	assert.Equal(t, "Read", groups[2].DisplayName)
	assert.Len(t, groups[2].Entries, 1)
}

// --- Phase 4: Transcript rendering tests ---

func TestRenderBlock_User(t *testing.T) {
	block := tui.TranscriptBlock{
		Type:      tui.BlockUser,
		TextParts: []string{"what files handle tool rendering?"},
	}
	result := tui.RenderTranscriptBlock(block, 80, false)
	stripped := stripAnsi(result)
	assert.Contains(t, stripped, ">")
	assert.Contains(t, stripped, "what files handle tool rendering?")
}

func TestRenderBlock_AssistantText(t *testing.T) {
	block := tui.TranscriptBlock{
		Type:      tui.BlockAssistant,
		AgentName: "coder",
		TextParts: []string{"The tool rendering is handled by agent_panel.go."},
	}
	result := tui.RenderTranscriptBlock(block, 80, false)
	stripped := stripAnsi(result)
	assert.Contains(t, stripped, "⏺")
	assert.Contains(t, stripped, "The tool rendering is handled by agent_panel.go.")
}

func TestRenderBlock_ToolCollapsed(t *testing.T) {
	block := tui.TranscriptBlock{
		Type:      tui.BlockAssistant,
		AgentName: "coder",
		Tools: []tui.ToolEntry{
			{
				Call:   tui.ToolCall{Name: "read_file", Content: `{"path":"app.go"}`, Status: tui.ToolRunning},
				Result: &tui.ToolCall{Name: "read_file", Content: "line1\nline2\nline3\n", Status: tui.ToolDone},
			},
		},
	}
	result := tui.RenderTranscriptBlock(block, 80, false)
	stripped := stripAnsi(result)
	assert.Contains(t, stripped, "⏺")
	assert.Contains(t, stripped, "Read(app.go)")
	assert.Contains(t, stripped, "⎿")
	assert.Contains(t, stripped, "3 lines")
}

func TestRenderBlock_ToolExpanded(t *testing.T) {
	block := tui.TranscriptBlock{
		Type:      tui.BlockAssistant,
		AgentName: "coder",
		Tools: []tui.ToolEntry{
			{
				Call:   tui.ToolCall{Name: "read_file", Content: `{"path":"app.go"}`, Status: tui.ToolRunning},
				Result: &tui.ToolCall{Name: "read_file", Content: "package tui\n\nfunc main() {}\n", Status: tui.ToolDone},
			},
		},
	}
	result := tui.RenderTranscriptBlock(block, 80, true)
	stripped := stripAnsi(result)
	assert.Contains(t, stripped, "Read(app.go)")
	assert.Contains(t, stripped, "package tui")
	assert.NotContains(t, stripped, "(ctrl+o to expand)")
}

func TestRenderBlock_ToolRunning(t *testing.T) {
	block := tui.TranscriptBlock{
		Type:      tui.BlockAssistant,
		AgentName: "coder",
		Tools: []tui.ToolEntry{
			{
				Call: tui.ToolCall{Name: "read_file", Content: `{"path":"app.go"}`, Status: tui.ToolRunning},
			},
		},
	}
	result := tui.RenderTranscriptBlock(block, 80, false)
	stripped := stripAnsi(result)
	assert.Contains(t, stripped, "Read(app.go)")
	assert.Contains(t, stripped, "⟳")
}

func TestRenderBlock_ToolGroup(t *testing.T) {
	block := tui.TranscriptBlock{
		Type:      tui.BlockAssistant,
		AgentName: "coder",
		Tools: []tui.ToolEntry{
			{
				Call:   tui.ToolCall{Name: "read_file", Content: `{"path":"a.go"}`, Status: tui.ToolRunning},
				Result: &tui.ToolCall{Name: "read_file", Content: "x", Status: tui.ToolDone},
			},
			{
				Call:   tui.ToolCall{Name: "read_file", Content: `{"path":"b.go"}`, Status: tui.ToolRunning},
				Result: &tui.ToolCall{Name: "read_file", Content: "y", Status: tui.ToolDone},
			},
			{
				Call:   tui.ToolCall{Name: "read_file", Content: `{"path":"c.go"}`, Status: tui.ToolRunning},
				Result: &tui.ToolCall{Name: "read_file", Content: "z", Status: tui.ToolDone},
			},
		},
	}
	// Collapsed: should show group summary
	result := tui.RenderTranscriptBlock(block, 80, false)
	stripped := stripAnsi(result)
	assert.Contains(t, stripped, "Read 3 files")

	// Expanded: should show individual tools
	resultExp := tui.RenderTranscriptBlock(block, 80, true)
	strippedExp := stripAnsi(resultExp)
	assert.Contains(t, strippedExp, "Read(a.go)")
	assert.Contains(t, strippedExp, "Read(b.go)")
	assert.Contains(t, strippedExp, "Read(c.go)")
}

func TestRenderBlock_Status(t *testing.T) {
	block := tui.TranscriptBlock{
		Type:       tui.BlockStatus,
		StatusText: "What model do you want?",
	}
	result := tui.RenderTranscriptBlock(block, 80, false)
	stripped := stripAnsi(result)
	assert.Contains(t, stripped, "What model do you want?")
}

func TestRenderBlock_AssistantWithStatusText(t *testing.T) {
	block := tui.TranscriptBlock{
		Type:       tui.BlockAssistant,
		AgentName:  "coder",
		TextParts:  []string{"Done analyzing."},
		StatusText: "✻ Crunched for 2s",
	}
	result := tui.RenderTranscriptBlock(block, 80, false)
	stripped := stripAnsi(result)
	assert.Contains(t, stripped, "Done analyzing.")
	assert.Contains(t, stripped, "✻ Crunched for 2s")
}

func TestRenderBlock_ToolError(t *testing.T) {
	block := tui.TranscriptBlock{
		Type:      tui.BlockAssistant,
		AgentName: "coder",
		Tools: []tui.ToolEntry{
			{
				Call:   tui.ToolCall{Name: "bash", Content: `{"command":"exit 1"}`, Status: tui.ToolRunning},
				Result: &tui.ToolCall{Name: "bash", Content: "exit status 1", Status: tui.ToolError},
			},
		},
	}
	result := tui.RenderTranscriptBlock(block, 80, false)
	stripped := stripAnsi(result)
	assert.Contains(t, stripped, "Bash(exit 1)")
	assert.Contains(t, stripped, "✗")
}

// --- Phase 7: Transcript search tests ---

func TestSearchBlocks_FindsText(t *testing.T) {
	blocks := []tui.TranscriptBlock{
		{Type: tui.BlockUser, TextParts: []string{"hello world"}},
		{Type: tui.BlockAssistant, AgentName: "coder", TextParts: []string{"I found the auth module"}},
		{Type: tui.BlockUser, TextParts: []string{"thanks"}},
	}
	matches := tui.SearchTranscriptBlocks(blocks, "auth")
	require.Len(t, matches, 1)
	assert.Equal(t, 1, matches[0])
}

func TestSearchBlocks_CaseInsensitive(t *testing.T) {
	blocks := []tui.TranscriptBlock{
		{Type: tui.BlockAssistant, AgentName: "coder", TextParts: []string{"The ToolCall struct"}},
	}
	matches := tui.SearchTranscriptBlocks(blocks, "toolcall")
	require.Len(t, matches, 1)
	assert.Equal(t, 0, matches[0])
}

func TestSearchBlocks_SearchesToolContent(t *testing.T) {
	blocks := []tui.TranscriptBlock{
		{Type: tui.BlockAssistant, AgentName: "coder", Tools: []tui.ToolEntry{
			{
				Call:   tui.ToolCall{Name: "read_file", Content: `{"path":"auth.go"}`},
				Result: &tui.ToolCall{Content: "package auth\nfunc Login() {}"},
			},
		}},
	}
	// Search in tool input
	matches := tui.SearchTranscriptBlocks(blocks, "auth.go")
	require.Len(t, matches, 1)

	// Search in tool output
	matches = tui.SearchTranscriptBlocks(blocks, "Login")
	require.Len(t, matches, 1)
}

func TestSearchBlocks_NoMatch(t *testing.T) {
	blocks := []tui.TranscriptBlock{
		{Type: tui.BlockUser, TextParts: []string{"hello"}},
	}
	matches := tui.SearchTranscriptBlocks(blocks, "nonexistent")
	assert.Empty(t, matches)
}

func TestSearchBlocks_MultipleMatches(t *testing.T) {
	blocks := []tui.TranscriptBlock{
		{Type: tui.BlockUser, TextParts: []string{"fix the auth bug"}},
		{Type: tui.BlockAssistant, TextParts: []string{"Looking at auth module"}},
		{Type: tui.BlockUser, TextParts: []string{"great, now test it"}},
		{Type: tui.BlockAssistant, TextParts: []string{"Auth tests pass"}},
	}
	matches := tui.SearchTranscriptBlocks(blocks, "auth")
	assert.Len(t, matches, 3) // blocks 0, 1, 3
	assert.Equal(t, []int{0, 1, 3}, matches)
}
