package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func TestRenderStatusBar(t *testing.T) {
	result := tui.RenderStatusBar("claude-sonnet-4", 1500, 60)
	assert.Contains(t, result, "claude-sonnet-4")
	assert.Contains(t, result, "1.5k")
}

func TestRenderStatusBarMillions(t *testing.T) {
	result := tui.RenderStatusBar("claude-sonnet-4", 1500000, 60)
	assert.Contains(t, result, "1.5m")
}

func TestRenderStatusBarSmall(t *testing.T) {
	result := tui.RenderStatusBar("claude-sonnet-4", 42, 60)
	assert.Contains(t, result, "42")
}
