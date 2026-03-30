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

func TestRenderPromptEmpty(t *testing.T) {
	result := tui.RenderPrompt("", 60)
	assert.Contains(t, result, "█")
}
