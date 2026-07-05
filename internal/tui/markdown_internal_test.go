package tui

import (
	"testing"

	"github.com/charmbracelet/glamour"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRendererCache_ReusesRendererPerWidth(t *testing.T) {
	calls := 0
	c := newRendererCache()
	c.build = func(width int) (*glamour.TermRenderer, error) {
		calls++
		return buildRenderer(width)
	}

	_, err := c.get(80)
	require.NoError(t, err)
	_, err = c.get(80)
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "same width must reuse the built renderer")

	_, err = c.get(100)
	require.NoError(t, err)
	assert.Equal(t, 2, calls, "a new width builds a new renderer")
}
