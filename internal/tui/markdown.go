package tui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// RenderMarkdown renders markdown content for terminal display.
func RenderMarkdown(content string, width int) string {
	if content == "" {
		return ""
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return content
	}

	rendered, err := r.Render(content)
	if err != nil {
		return content
	}

	return strings.TrimRight(rendered, "\n")
}
