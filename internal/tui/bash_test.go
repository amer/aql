package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func TestIsBashCommand(t *testing.T) {
	assert.True(t, tui.IsBashCommand("!ls"))
	assert.True(t, tui.IsBashCommand("! ls -la"))
	assert.True(t, tui.IsBashCommand("!echo hello"))
	assert.False(t, tui.IsBashCommand("ls"))
	assert.False(t, tui.IsBashCommand("/help"))
	assert.False(t, tui.IsBashCommand(""))
}

func TestParseBashCommand(t *testing.T) {
	assert.Equal(t, "ls", tui.ParseBashCommand("!ls"))
	assert.Equal(t, "ls -la", tui.ParseBashCommand("! ls -la"))
	assert.Equal(t, "echo hello", tui.ParseBashCommand("!echo hello"))
	assert.Equal(t, "", tui.ParseBashCommand("!"))
	assert.Equal(t, "", tui.ParseBashCommand("! "))
}
