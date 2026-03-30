package tui

import (
	"strings"
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
	}
}

// FilterCommands filters commands by prefix match against the query.
func FilterCommands(cmds []Command, query string) []Command {
	if query == "" {
		return cmds
	}
	q := strings.ToLower(query)
	var result []Command
	for _, cmd := range cmds {
		if strings.HasPrefix(strings.ToLower(cmd.Name), q) {
			result = append(result, cmd)
		}
	}
	return result
}

// RenderModelPicker renders the interactive model selection list.
func RenderModelPicker(models []ModelOption, selected int, currentID string, width int) string {
	if len(models) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, BoldStyle.Render("Select model:")+" "+DimStyle.Render("(↑↓ navigate, enter select, esc cancel)"))
	for i, m := range models {
		current := ""
		if m.ID == currentID {
			current = DimStyle.Render(" (current)")
		}
		line := "  " + m.DisplayName + DimStyle.Render("  "+m.ID) + current
		if i == selected {
			line = PaletteSelectedStyle.Render("▸ "+m.DisplayName) + DimStyle.Render("  "+m.ID) + current
		}
		lines = append(lines, line)
	}

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
