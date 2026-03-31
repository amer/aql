package tui

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
)

// markdownStyle is the dark style with zero document margin so the caller
// controls all indentation via transcriptPadding / transcriptIndent.
var markdownStyle = func() glamour.TermRendererOption {
	s := styles.DarkStyleConfig
	zero := uint(0)
	s.Document.Margin = &zero
	return glamour.WithStyles(s)
}()

// RenderMarkdown renders markdown content for terminal display.
func RenderMarkdown(content string, width int) string {
	if content == "" {
		return ""
	}

	r, err := glamour.NewTermRenderer(
		markdownStyle,
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return content
	}

	rendered, err := r.Render(content)
	if err != nil {
		return content
	}

	return strings.Trim(rendered, "\n")
}
