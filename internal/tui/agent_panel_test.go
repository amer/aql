package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func TestRenderAgentHeaderActive(t *testing.T) {
	result := tui.RenderAgentHeader("coder", tui.AgentActive)
	assert.Contains(t, result, "●")
	assert.Contains(t, result, "coder")
}

func TestRenderAgentHeaderWaiting(t *testing.T) {
	result := tui.RenderAgentHeader("doc-writer", tui.AgentWaiting)
	assert.Contains(t, result, "○")
	assert.Contains(t, result, "doc-writer")
}

func TestRenderAgentHeaderDone(t *testing.T) {
	result := tui.RenderAgentHeader("reviewer", tui.AgentDone)
	assert.Contains(t, result, "✓")
}

func TestRenderAgentHeaderError(t *testing.T) {
	result := tui.RenderAgentHeader("coder", tui.AgentError)
	assert.Contains(t, result, "✗")
}

func TestRenderToolBlock(t *testing.T) {
	tc := tui.ToolCall{Name: "write_file", Content: "internal/auth/auth.go"}
	result := tui.RenderToolBlock(tc)
	assert.Contains(t, result, "write_file")
	assert.Contains(t, result, "auth.go")
}

func TestRenderToolBlockEmpty(t *testing.T) {
	tc := tui.ToolCall{Name: "bash", Content: ""}
	result := tui.RenderToolBlock(tc)
	assert.Contains(t, result, "(no output)")
}

func TestRenderAgentPanel(t *testing.T) {
	data := tui.AgentPanelData{
		Name:   "coder",
		Status: tui.AgentActive,
		Output: "Writing test for user auth...",
		ToolCalls: []tui.ToolCall{
			{Name: "write_file", Content: "internal/auth/auth_test.go"},
		},
	}

	result := tui.RenderAgentPanel(data)
	assert.Contains(t, result, "coder")
	assert.Contains(t, result, "Writing test")
	assert.Contains(t, result, "write_file")
}

func TestRenderAgentPanelWithStatusMsg(t *testing.T) {
	data := tui.AgentPanelData{
		Name:      "doc-writer",
		Status:    tui.AgentWaiting,
		StatusMsg: "waiting for code_written event",
	}

	result := tui.RenderAgentPanel(data)
	assert.Contains(t, result, "waiting for code_written")
}

func TestRenderAgentPanelMinimal(t *testing.T) {
	data := tui.AgentPanelData{
		Name:   "reviewer",
		Status: tui.AgentDone,
	}

	result := tui.RenderAgentPanel(data)
	assert.Contains(t, result, "reviewer")
	assert.Contains(t, result, "✓")
}
