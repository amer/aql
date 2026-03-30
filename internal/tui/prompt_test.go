package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func TestRenderPrompt(t *testing.T) {
	result := tui.RenderPrompt("hello world", 60)
	assert.Contains(t, result, "hello world")
}

func TestRenderPromptCursorPrefix(t *testing.T) {
	result := tui.RenderPrompt("test", 60)
	plain := stripAnsi(result)
	assert.Contains(t, plain, "❯ test")
}

func TestRenderPromptEmpty(t *testing.T) {
	// RenderPrompt no longer adds cursor — caller provides it via InputBuffer.RenderWithCursor()
	buf := tui.NewInputBuffer()
	result := tui.RenderPrompt(buf.RenderWithCursor(), 60)
	assert.Contains(t, result, "█")
}

func TestRenderPromptSeparatorAbove(t *testing.T) {
	result := tui.RenderPromptArea("test input", "aql-project", 60)
	plain := stripAnsi(result)
	// Should contain the project badge
	assert.Contains(t, plain, "aql-project")
	// Should contain the cursor prefix
	assert.Contains(t, plain, "❯ test input")
}

func TestRenderPromptAreaShowsProjectBadge(t *testing.T) {
	result := tui.RenderPromptArea("", "my-project", 60)
	plain := stripAnsi(result)
	assert.Contains(t, plain, "my-project")
}

func TestRenderStatusBarClaude(t *testing.T) {
	result := tui.RenderStatusBar("claude-haiku-4-5", 1500, 60)
	plain := stripAnsi(result)
	// Left side: agent info
	assert.Contains(t, plain, "agents on")
	// Right side: token usage percentage
	assert.Contains(t, plain, "auto-compact")
}
