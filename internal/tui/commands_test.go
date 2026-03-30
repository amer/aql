package tui_test

import (
	"regexp"
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	filtered := tui.FilterCommands(cmds, "/exit")
	found := false
	for _, cmd := range filtered {
		if cmd.Name == "/exit" {
			found = true
		}
	}
	assert.True(t, found, "/exit should be in fuzzy results for '/exit'")
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

func TestModelTiers(t *testing.T) {
	tiers := tui.ModelTiers()
	require.Len(t, tiers, 3, "should have Default, Opus, Haiku")

	assert.Equal(t, "Default (recommended)", tiers[0].Label)
	assert.Equal(t, "Opus", tiers[1].Label)
	assert.Equal(t, "Haiku", tiers[2].Label)

	for _, tier := range tiers {
		assert.NotEmpty(t, tier.ModelID, "tier must have a model ID")
		assert.NotEmpty(t, tier.Description, "tier must have a description")
		assert.NotEmpty(t, tier.Pricing, "tier must have pricing")
	}
}

func TestRenderModelPicker(t *testing.T) {
	tiers := tui.ModelTiers()
	result := tui.RenderModelPicker(tiers, 0, "claude-sonnet-4-6", 80)
	plain := stripAnsiCmds(result)

	assert.Contains(t, plain, "Select model")
	assert.Contains(t, plain, "Default (recommended)")
	assert.Contains(t, plain, "Opus")
	assert.Contains(t, plain, "Haiku")
	assert.Contains(t, plain, "per Mtok")
	assert.Contains(t, plain, "Enter to confirm")
	assert.Contains(t, plain, "Esc to exit")
}

func TestRenderModelPickerHighlightsCurrent(t *testing.T) {
	tiers := tui.ModelTiers()
	result := tui.RenderModelPicker(tiers, 1, "claude-opus-4-6", 80)
	plain := stripAnsiCmds(result)
	// Opus is selected (idx 1) and is current model
	assert.Contains(t, plain, "✓")
}

func TestRenderModelPickerDifferentSelection(t *testing.T) {
	tiers := tui.ModelTiers()
	r0 := tui.RenderModelPicker(tiers, 0, "", 80)
	r1 := tui.RenderModelPicker(tiers, 1, "", 80)
	assert.NotEqual(t, r0, r1, "different selection should render differently")
}

func TestRenderModelPickerCustomEntry(t *testing.T) {
	tiers := tui.ModelTiers()
	// selected=3 means "Use custom model ID" entry
	result := tui.RenderModelPicker(tiers, 3, "", 80)
	plain := stripAnsiCmds(result)
	assert.Contains(t, plain, "custom model ID")
}

var stripAnsiCmds = func() func(string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return func(s string) string { return re.ReplaceAllString(s, "") }
}()
