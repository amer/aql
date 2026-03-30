package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func TestFormatTokenCount(t *testing.T) {
	assert.Equal(t, "0", tui.FormatTokenCount(0))
	assert.Equal(t, "500", tui.FormatTokenCount(500))
	assert.Equal(t, "1,000", tui.FormatTokenCount(1000))
	assert.Equal(t, "15,432", tui.FormatTokenCount(15432))
	assert.Equal(t, "1,000,000", tui.FormatTokenCount(1000000))
	assert.Equal(t, "1,234,567", tui.FormatTokenCount(1234567))
}

func TestFormatTokenCountShort(t *testing.T) {
	assert.Equal(t, "0", tui.FormatTokenCountShort(0))
	assert.Equal(t, "500", tui.FormatTokenCountShort(500))
	assert.Equal(t, "1.0k", tui.FormatTokenCountShort(1000))
	assert.Equal(t, "15.4k", tui.FormatTokenCountShort(15432))
	assert.Equal(t, "1.0m", tui.FormatTokenCountShort(1000000))
	assert.Equal(t, "1.2m", tui.FormatTokenCountShort(1234567))
}
