package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func TestSlashCommands(t *testing.T) {
	cmds := tui.SlashCommands()
	assert.True(t, len(cmds) > 0, "should have at least one command")

	// All commands must start with /
	for _, cmd := range cmds {
		assert.True(t, len(cmd.Name) > 0, "command name must not be empty")
		assert.Equal(t, "/", cmd.Name[:1], "command must start with /")
		assert.NotEmpty(t, cmd.Description, "command must have a description")
	}
}

func TestFilterCommandsEmpty(t *testing.T) {
	cmds := tui.SlashCommands()
	filtered := tui.FilterCommands(cmds, "")
	assert.Equal(t, cmds, filtered, "empty filter returns all commands")
}

func TestFilterCommandsPartialMatch(t *testing.T) {
	cmds := tui.SlashCommands()
	filtered := tui.FilterCommands(cmds, "/e")
	for _, cmd := range filtered {
		assert.Contains(t, cmd.Name, "e", "filtered commands should contain the query")
	}
}

func TestFilterCommandsFullMatch(t *testing.T) {
	cmds := tui.SlashCommands()
	filtered := tui.FilterCommands(cmds, "/exit")
	found := false
	for _, cmd := range filtered {
		if cmd.Name == "/exit" {
			found = true
		}
	}
	assert.True(t, found, "should find /exit command")
}

func TestFilterCommandsNoMatch(t *testing.T) {
	cmds := tui.SlashCommands()
	filtered := tui.FilterCommands(cmds, "/zzzznotacommand")
	assert.Empty(t, filtered, "no commands should match gibberish")
}

func TestFilterCommandsCaseInsensitive(t *testing.T) {
	cmds := tui.SlashCommands()
	lower := tui.FilterCommands(cmds, "/h")
	upper := tui.FilterCommands(cmds, "/H")
	assert.Equal(t, lower, upper, "filter should be case insensitive")
}

func TestRenderCommandPalette(t *testing.T) {
	cmds := []tui.Command{
		{Name: "/exit", Description: "Exit AQL"},
		{Name: "/help", Description: "Show help"},
	}
	result := tui.RenderCommandPalette(cmds, 0, 60)
	assert.Contains(t, result, "/exit")
	assert.Contains(t, result, "/help")
	assert.Contains(t, result, "Exit AQL")
}

func TestRenderCommandPaletteHighlight(t *testing.T) {
	cmds := []tui.Command{
		{Name: "/exit", Description: "Exit AQL"},
		{Name: "/help", Description: "Show help"},
	}
	r0 := tui.RenderCommandPalette(cmds, 0, 60)
	r1 := tui.RenderCommandPalette(cmds, 1, 60)
	// Different selected index should produce different output
	assert.NotEqual(t, r0, r1, "different selection should render differently")
}

func TestRenderCommandPaletteEmpty(t *testing.T) {
	result := tui.RenderCommandPalette(nil, 0, 60)
	assert.Empty(t, result, "empty command list should render empty")
}

func TestCommandAction(t *testing.T) {
	cmds := tui.SlashCommands()
	for _, cmd := range cmds {
		assert.NotEmpty(t, cmd.Action, "command %s must have an action", cmd.Name)
	}
}
