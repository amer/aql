package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func TestRenderSpinner(t *testing.T) {
	result := tui.RenderSpinner(0, "thinking...")
	assert.Contains(t, result, "thinking...")
}

func TestRenderSpinnerWraps(t *testing.T) {
	r1 := tui.RenderSpinner(0, "test")
	r2 := tui.RenderSpinner(10, "test")
	assert.Equal(t, r1, r2, "frame 10 should wrap to frame 0")
}
