package tui_test

import (
	"regexp"
	"testing"

	"github.com/amer/aql/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
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

func TestModelTiersFallback(t *testing.T) {
	tiers := tui.DefaultModelTiers()
	require.Len(t, tiers, 3, "should have Sonnet, Opus, Haiku")

	assert.Contains(t, tiers[0].Label, "Sonnet")
	assert.Contains(t, tiers[1].Label, "Opus")
	assert.Contains(t, tiers[2].Label, "Haiku")

	for _, tier := range tiers {
		assert.NotEmpty(t, tier.ModelID, "tier must have a model ID")
		assert.NotEmpty(t, tier.Description, "tier must have a description")
	}
}

func TestSetModelTiers(t *testing.T) {
	m := tui.NewModel("test", []string{"agent"}, nil)

	dynamic := []tui.ModelTier{
		{Label: "Opus 4.6", ModelID: "claude-opus-4-6-20260301", Description: "1M ctx"},
		{Label: "Sonnet 4.6", ModelID: "claude-sonnet-4-6-20260301", Description: "200k ctx"},
	}
	m.SetModelTiers(dynamic)
	assert.Equal(t, dynamic, m.GetModelTiers())
}

func TestModelPickerUsesDynamicTiers(t *testing.T) {
	m := tui.NewModel("test", []string{"agent"}, nil)

	dynamic := []tui.ModelTier{
		{Label: "Custom Model A", ModelID: "model-a", Description: "fast"},
		{Label: "Custom Model B", ModelID: "model-b", Description: "smart"},
	}
	m.SetModelTiers(dynamic)

	// Trigger /model command
	result := tui.RenderModelPicker(m.GetModelTiers(), 0, "", 80)
	plain := stripAnsiCmds(result)
	assert.Contains(t, plain, "Custom Model A")
	assert.Contains(t, plain, "Custom Model B")
	assert.NotContains(t, plain, "Haiku", "should not show hardcoded tiers")
}

func TestModelsLoadedMsgUpdatesTiers(t *testing.T) {
	m := tui.NewModel("test", []string{"agent"}, nil)

	// Initially uses defaults
	assert.Equal(t, tui.DefaultModelTiers(), m.GetModelTiers())

	// Simulate background probe completing
	newTiers := []tui.ModelTier{
		{Label: "Opus 4.6", ModelID: "claude-opus-4-6", Description: "1000k ctx"},
		{Label: "Sonnet 4.6", ModelID: "claude-sonnet-4-6", Description: "200k ctx"},
	}
	updated, _ := m.Update(tui.ModelsLoadedMsg{Tiers: newTiers})
	m = updated.(tui.Model)

	assert.Equal(t, newTiers, m.GetModelTiers())
}

func TestBootstrappingIsInvisible(t *testing.T) {
	m := tui.NewModel("test", []string{"agent"}, nil)

	// Bootstrapping is always invisible
	assert.False(t, m.IsBootstrapping())
	m.SetBootstrapping(true)
	assert.False(t, m.IsBootstrapping(), "bootstrapping should be invisible")

	// View should NOT contain "Bootstrapping"
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = updated.(tui.Model)
	view := m.View()
	plain := stripAnsiCmds(view)
	assert.NotContains(t, plain, "Bootstrapping")

	// ModelsLoadedMsg should still update model tiers silently
	tiers := tui.DefaultModelTiers()
	updated2, _ := m.Update(tui.ModelsLoadedMsg{Tiers: tiers})
	m = updated2.(tui.Model)
	assert.Equal(t, tiers, m.GetModelTiers())
}

func TestRenderModelPicker(t *testing.T) {
	tiers := tui.DefaultModelTiers()
	result := tui.RenderModelPicker(tiers, 0, "claude-sonnet-4-6", 80)
	plain := stripAnsiCmds(result)

	assert.Contains(t, plain, "Select model")
	assert.Contains(t, plain, "Sonnet")
	assert.Contains(t, plain, "Opus")
	assert.Contains(t, plain, "Haiku")
	assert.Contains(t, plain, "Enter to confirm")
	assert.Contains(t, plain, "Esc to exit")
}

func TestRenderModelPickerHighlightsCurrent(t *testing.T) {
	tiers := tui.DefaultModelTiers()
	result := tui.RenderModelPicker(tiers, 1, "claude-opus-4-6", 80)
	plain := stripAnsiCmds(result)
	// Opus is selected (idx 1) and is current model
	assert.Contains(t, plain, "✓")
}

func TestRenderModelPickerDifferentSelection(t *testing.T) {
	tiers := tui.DefaultModelTiers()
	r0 := tui.RenderModelPicker(tiers, 0, "", 80)
	r1 := tui.RenderModelPicker(tiers, 1, "", 80)
	assert.NotEqual(t, r0, r1, "different selection should render differently")
}

func TestRenderModelPickerCustomEntry(t *testing.T) {
	tiers := tui.DefaultModelTiers()
	// selected=3 means "Use custom model ID" entry
	result := tui.RenderModelPicker(tiers, 3, "", 80)
	plain := stripAnsiCmds(result)
	assert.Contains(t, plain, "custom model ID")
}

var stripAnsiCmds = func() func(string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return func(s string) string { return re.ReplaceAllString(s, "") }
}()
