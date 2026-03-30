package tui_test

import (
	"regexp"
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func stripANSIPrompt(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

func TestRenderPrompt(t *testing.T) {
	result := tui.RenderPrompt("hello world", 60)
	assert.Contains(t, result, "hello world")
}

func TestRenderPromptCursorPrefix(t *testing.T) {
	result := tui.RenderPrompt("test", 60)
	plain := stripANSIPrompt(result)
	assert.Contains(t, plain, ") test")
}

func TestRenderPromptEmpty(t *testing.T) {
	result := tui.RenderPrompt("", 60)
	assert.Contains(t, result, "█")
}

func TestRenderPromptStreaming(t *testing.T) {
	result := tui.RenderPromptStreaming(0, "coder", 60)
	assert.Contains(t, result, "coder")
	assert.Contains(t, result, "responding")
}

func TestRenderPromptSeparatorAbove(t *testing.T) {
	result := tui.RenderPromptArea("test input", "aql-project", 60)
	plain := stripANSIPrompt(result)
	// Should contain the project badge
	assert.Contains(t, plain, "aql-project")
	// Should contain the cursor prefix
	assert.Contains(t, plain, ") test input")
}

func TestRenderPromptSeparatorBelow(t *testing.T) {
	result := tui.RenderPromptArea("", "my-project", 60)
	plain := stripANSIPrompt(result)
	// The horizontal lines should appear (rendered as ─ characters)
	assert.Contains(t, plain, "─")
}

func TestRenderStatusBarClaude(t *testing.T) {
	result := tui.RenderStatusBar("claude-haiku-4-5", 1500, 60)
	plain := stripANSIPrompt(result)
	// Left side: agent info
	assert.Contains(t, plain, "agents on")
	// Right side: token usage percentage
	assert.Contains(t, plain, "auto-compact")
}
