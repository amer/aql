package tui

import (
	"fmt"
	"strings"

	"github.com/sahilm/fuzzy"
)

// Command represents a slash command in the palette.
type Command struct {
	Name        string
	Description string
	Action      string // action identifier used by the app to dispatch
}

// SlashCommands returns the list of available slash commands.
func SlashCommands() []Command {
	return []Command{
		{Name: "/help", Description: "Show available commands", Action: "help"},
		{Name: "/exit", Description: "Exit AQL", Action: "exit"},
		{Name: "/quit", Description: "Exit AQL", Action: "exit"},
		{Name: "/clear", Description: "Clear chat history", Action: "clear"},
		{Name: "/agents", Description: "List active agents", Action: "agents"},
		{Name: "/status", Description: "Show workflow status", Action: "status"},
		{Name: "/model", Description: "List/switch models (e.g. /model sonnet)", Action: "model"},
		{Name: "/compact", Description: "Compact conversation context", Action: "compact"},
		{Name: "/spinner", Description: "Cycle spinner animation style", Action: "spinner"},
	}
}

// FilterCommands filters commands by fuzzy match against the query.
func FilterCommands(cmds []Command, query string) []Command {
	if query == "" {
		return cmds
	}
	// Build searchable strings: "name description"
	strs := make([]string, len(cmds))
	for i, cmd := range cmds {
		strs[i] = cmd.Name + " " + cmd.Description
	}
	matches := fuzzy.Find(query, strs)
	result := make([]Command, len(matches))
	for i, m := range matches {
		result[i] = cmds[m.Index]
	}
	return result
}

// ModelTier represents a hardcoded model tier (like Claude Code's model picker).
type ModelTier struct {
	Label       string // e.g. "Default (recommended)", "Opus", "Haiku"
	ModelID     string // full model ID sent to the API
	Description string // e.g. "Most capable for complex work"
	Pricing     string // e.g. "$5/$25 per Mtok"
}

// DefaultModelTiers returns the hardcoded fallback model tiers.
func DefaultModelTiers() []ModelTier {
	return []ModelTier{
		{
			Label:       "Default (recommended)",
			ModelID:     "claude-sonnet-4-6",
			Description: "Use the default model (currently Sonnet 4.6)",
			Pricing:     "$3/$15 per Mtok",
		},
		{
			Label:       "Opus",
			ModelID:     "claude-opus-4-6",
			Description: "Most capable for complex work",
			Pricing:     "$5/$25 per Mtok",
		},
		{
			Label:       "Haiku",
			ModelID:     "claude-haiku-4-5",
			Description: "Fastest for quick answers",
			Pricing:     "$1/$5 per Mtok",
		},
	}
}

// RenderModelPicker renders the model selection list matching Claude Code's style.
func RenderModelPicker(tiers []ModelTier, selected int, currentID string, width int) string {
	var lines []string

	// Header
	lines = append(lines, BoldStyle.Render("Select model"))
	lines = append(lines, DimStyle.Render("Switch between Claude models. For other/preview models, specify with --model."))
	lines = append(lines, "")

	for i, tier := range tiers {
		num := fmt.Sprintf("%d. ", i+1)
		current := ""
		if tier.ModelID == currentID {
			current = " ✓"
		}
		detail := DimStyle.Render(tier.Description + " · " + tier.Pricing)
		if i == selected {
			line := PaletteSelectedStyle.Render("❯ "+num+tier.Label+current) + "  " + detail
			lines = append(lines, line)
		} else {
			line := "  " + num + tier.Label + current + "  " + detail
			lines = append(lines, line)
		}
	}

	// "Use custom model ID" entry
	customLine := "  " + fmt.Sprintf("%d. ", len(tiers)+1) + DimStyle.Render("Use custom model ID")
	if selected == len(tiers) {
		customLine = PaletteSelectedStyle.Render("❯ "+fmt.Sprintf("%d. ", len(tiers)+1)+"Use custom model ID") + "  " + DimStyle.Render("specify with --model")
	}
	lines = append(lines, customLine)

	lines = append(lines, "")
	lines = append(lines, DimStyle.Render("Enter to confirm · Esc to exit"))

	content := strings.Join(lines, "\n")
	return PaletteBorderStyle.Width(width - 4).Render(content)
}

// RenderCommandPalette renders the command palette popup above the prompt.
func RenderCommandPalette(cmds []Command, selected int, width int) string {
	if len(cmds) == 0 {
		return ""
	}

	var lines []string
	for i, cmd := range cmds {
		name := cmd.Name
		desc := DimStyle.Render(cmd.Description)
		line := "  " + name + "  " + desc

		if i == selected {
			line = PaletteSelectedStyle.Render("▸ " + name + "  " + cmd.Description)
		}

		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")
	return PaletteBorderStyle.Width(width - 4).Render(content)
}
