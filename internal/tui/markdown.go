package tui

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - RenderMarkdown — renders markdown for terminal using glamour,
//     markdownStyle config.
//   - rendererCache / newRendererCache / buildRenderer — memoize the
//     expensive glamour renderer by width so View() doesn't rebuild it
//     per text part per frame.
//
// MUST NOT GO HERE:
//   - Custom markdown parsing, state mutation, agent imports.
//
// Q: Is the shared cache safe? View() runs on the single Bubble Tea
//    render goroutine, and the mutex guards the map; renderer reuse for
//    successive Render calls at a fixed width is glamour's intended use.
// ──────────────────────────────────────────────────────────────────

import (
	"strings"
	"sync"

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

// buildRenderer constructs a glamour renderer word-wrapped to width. This is
// the expensive step (it compiles the style config) H1 memoizes.
func buildRenderer(width int) (*glamour.TermRenderer, error) {
	return glamour.NewTermRenderer(markdownStyle, glamour.WithWordWrap(width))
}

// rendererCache memoizes one glamour renderer per wrap width. Terminal width
// changes rarely, so the map stays tiny; the mutex keeps it safe if a caller
// ever renders off the UI goroutine.
type rendererCache struct {
	mu      sync.Mutex
	byWidth map[int]*glamour.TermRenderer
	build   func(width int) (*glamour.TermRenderer, error)
}

func newRendererCache() *rendererCache {
	return &rendererCache{byWidth: map[int]*glamour.TermRenderer{}, build: buildRenderer}
}

func (c *rendererCache) get(width int) (*glamour.TermRenderer, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if r, ok := c.byWidth[width]; ok {
		return r, nil
	}
	r, err := c.build(width)
	if err != nil {
		return nil, err
	}
	c.byWidth[width] = r
	return r, nil
}

// markdownCache is the process-wide renderer memo. It is a pure cache: every
// width maps to a deterministic renderer, so sharing it introduces no
// observable state — only avoids rebuilding on every frame.
var markdownCache = newRendererCache()

// RenderMarkdown renders markdown content for terminal display.
func RenderMarkdown(content string, width int) string {
	if content == "" {
		return ""
	}

	r, err := markdownCache.get(width)
	if err != nil {
		return content
	}

	rendered, err := r.Render(content)
	if err != nil {
		return content
	}

	return strings.Trim(rendered, "\n")
}
