package tui_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

// stripANSI removes ANSI escape sequences for test assertions.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func TestRenderMarkdownPlainText(t *testing.T) {
	result := tui.RenderMarkdown("hello world", 80)
	assert.Contains(t, stripANSI(result), "hello world")
}

func TestRenderMarkdownBold(t *testing.T) {
	result := tui.RenderMarkdown("this is **bold** text", 80)
	plain := stripANSI(result)
	assert.Contains(t, plain, "bold")
	assert.NotContains(t, plain, "**")
}

func TestRenderMarkdownInlineCode(t *testing.T) {
	result := tui.RenderMarkdown("run `go test` now", 80)
	plain := stripANSI(result)
	assert.Contains(t, plain, "go test")
	assert.NotContains(t, plain, "`")
}

func TestRenderMarkdownCodeBlock(t *testing.T) {
	input := "here is code:\n```go\nfunc main() {}\n```"
	result := tui.RenderMarkdown(input, 80)
	plain := stripANSI(result)
	assert.Contains(t, plain, "func main()")
}

func TestRenderMarkdownHeading(t *testing.T) {
	result := tui.RenderMarkdown("# Title\nsome text", 80)
	plain := stripANSI(result)
	assert.Contains(t, plain, "Title")
	assert.Contains(t, plain, "some text")
}

func TestRenderMarkdownBulletList(t *testing.T) {
	input := "items:\n- one\n- two\n- three"
	result := tui.RenderMarkdown(input, 80)
	plain := stripANSI(result)
	assert.Contains(t, plain, "one")
	assert.Contains(t, plain, "two")
	assert.Contains(t, plain, "three")
}

func TestRenderMarkdownNoLeadingMargin(t *testing.T) {
	result := tui.RenderMarkdown("hello world", 80)
	plain := stripANSI(result)
	assert.True(t, strings.HasPrefix(plain, "hello world"), "rendered text should have no leading margin")
}

func TestRenderMarkdownEmpty(t *testing.T) {
	result := tui.RenderMarkdown("", 80)
	assert.Empty(t, result)
}

func TestRenderMarkdownPreservesLinks(t *testing.T) {
	result := tui.RenderMarkdown("see [docs](https://example.com)", 80)
	plain := stripANSI(result)
	assert.Contains(t, plain, "docs")
}
