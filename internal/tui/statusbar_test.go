package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func TestRenderStatusBar(t *testing.T) {
	result := tui.RenderStatusBar("pair-programming", 3, 60)
	assert.Contains(t, result, "AQL")
	assert.Contains(t, result, "pair-programming")
	assert.Contains(t, result, "3 agents")
}

func TestRenderStatusBarNarrow(t *testing.T) {
	result := tui.RenderStatusBar("test", 1, 20)
	assert.Contains(t, result, "AQL")
}
