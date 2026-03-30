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

// RenderModelPicker renders the interactive model selection list with search and custom ID input.
func RenderModelPicker(models []ModelOption, selected int, currentID string, input string, width int) string {
	var lines []string

	// Header with search input
	header := BoldStyle.Render("Select model:")
	if input != "" {
		header += " " + input + "█"
	} else {
		header += " " + DimStyle.Render("type to filter, ↑↓ navigate, enter select, esc cancel")
	}
	lines = append(lines, header)

	for i, m := range models {
		ctx := formatContextWindow(m.MaxInputTokens)
		current := ""
		if m.ID == currentID {
			current = DimStyle.Render(" (current)")
		}
		detail := DimStyle.Render("  " + m.ID + "  " + ctx)
		line := "  " + m.DisplayName + detail + current
		if i == selected {
			line = PaletteSelectedStyle.Render("▸ "+m.DisplayName) + detail + current
		}
		lines = append(lines, line)
	}

	// "Use custom ID" entry at the bottom
	customLine := "  " + DimStyle.Render("Use custom model ID")
	if input != "" {
		customLine = "  " + DimStyle.Render("Use: "+input)
	}
	if selected == len(models) {
		if input != "" {
			customLine = PaletteSelectedStyle.Render("▸ Use: " + input)
		} else {
			customLine = PaletteSelectedStyle.Render("▸ Use custom model ID")
		}
	}
	lines = append(lines, customLine)

	content := strings.Join(lines, "\n")
	return PaletteBorderStyle.Width(width - 4).Render(content)
}

func formatContextWindow(tokens int64) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%.0fM ctx", float64(tokens)/1000000)
	}
	if tokens >= 1000 {
		return fmt.Sprintf("%dk ctx", tokens/1000)
	}
	return fmt.Sprintf("%d ctx", tokens)
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
