package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/domain"
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

func TestRenderToolBlockDone(t *testing.T) {
	tc := domain.ToolCall{Name: "write_file", Content: "internal/auth/auth.go", Status: domain.ToolDone}
	result := tui.RenderToolBlock(tc)
	assert.Contains(t, result, "write_file")
	assert.Contains(t, result, "auth.go")
	assert.Contains(t, result, "✓")
}

func TestRenderToolBlockRunning(t *testing.T) {
	tc := domain.ToolCall{Name: "bash", Content: "go test ./...", Status: domain.ToolRunning}
	result := tui.RenderToolBlock(tc)
	assert.Contains(t, result, "bash")
	assert.Contains(t, result, "⟳")
}

func TestRenderToolBlockError(t *testing.T) {
	tc := domain.ToolCall{Name: "bash", Content: "exit 1", Status: domain.ToolError}
	result := tui.RenderToolBlock(tc)
	assert.Contains(t, result, "✗")
}

func TestRenderToolBlockEmpty(t *testing.T) {
	tc := domain.ToolCall{Name: "bash", Content: ""}
	result := tui.RenderToolBlock(tc)
	assert.Contains(t, result, "(no output)")
}

func TestRenderAgentPanel(t *testing.T) {
	data := tui.AgentPanelData{
		Name:   "coder",
		Status: tui.AgentActive,
		Output: "Writing test for user auth...",
		ToolCalls: []domain.ToolCall{
			{Name: "write_file", Content: "auth_test.go", Status: domain.ToolDone},
		},
	}

	result := tui.RenderAgentPanel(data)
	assert.Contains(t, result, "coder")
	assert.Contains(t, result, "Writing test")
	assert.Contains(t, result, "write_file")
}

func TestRenderUserMessage(t *testing.T) {
	result := tui.RenderUserMessage("refactor auth module")
	assert.Contains(t, result, ">")
	assert.Contains(t, result, "refactor auth module")
}

func TestToolCall_ToolIDField(t *testing.T) {
	tc := domain.ToolCall{
		Name:    "read_file",
		Content: `{"path":"foo.go"}`,
		Status:  domain.ToolRunning,
		ToolID:  "toolu_abc123",
	}
	assert.Equal(t, "toolu_abc123", tc.ToolID)
	assert.Equal(t, "read_file", tc.Name)
}
