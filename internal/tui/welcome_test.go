package tui_test

import (
	"strings"
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

// truncatePath

func TestTruncatePath_ShortPath(t *testing.T) {
	result := tui.TruncatePath("/home/user/project", 40)
	assert.Equal(t, "/home/user/project", result)
}

func TestTruncatePath_LongPath(t *testing.T) {
	result := tui.TruncatePath("/Users/amer/Code/github.com/amer/aql", 30)
	assert.Contains(t, result, "…")
	assert.True(t, len(result) <= 30, "truncated path should fit within maxWidth")
	assert.True(t, strings.HasPrefix(result, "/"), "should start with /")
	assert.True(t, strings.HasSuffix(result, "aql"), "should keep last segment")
}

func TestTruncatePath_RootPath(t *testing.T) {
	result := tui.TruncatePath("/", 40)
	assert.Equal(t, "/", result)
}

func TestTruncatePath_HomeDir(t *testing.T) {
	result := tui.TruncatePath("/Users/amer", 40)
	assert.Equal(t, "/Users/amer", result)
}

func TestTruncatePath_SingleSegment(t *testing.T) {
	result := tui.TruncatePath("/verylongsinglesegment", 40)
	assert.Equal(t, "/verylongsinglesegment", result)
}

// ShortenHome

func TestShortenHome_ReplacesPrefix(t *testing.T) {
	result := tui.ShortenHome("/Users/amer/Code/project", "/Users/amer")
	assert.Equal(t, "~/Code/project", result)
}

func TestShortenHome_ExactHome(t *testing.T) {
	result := tui.ShortenHome("/Users/amer", "/Users/amer")
	assert.Equal(t, "~", result)
}

func TestShortenHome_NoMatch(t *testing.T) {
	result := tui.ShortenHome("/opt/data/project", "/Users/amer")
	assert.Equal(t, "/opt/data/project", result)
}

func TestShortenHome_EmptyHome(t *testing.T) {
	result := tui.ShortenHome("/Users/amer/Code", "")
	assert.Equal(t, "/Users/amer/Code", result)
}

// welcomeGreeting

func TestWelcomeGreeting_WithUsername(t *testing.T) {
	result := tui.WelcomeGreeting("amer")
	assert.Equal(t, "Welcome back amer!", result)
}

func TestWelcomeGreeting_EmptyUsername(t *testing.T) {
	result := tui.WelcomeGreeting("")
	assert.Equal(t, "Welcome back!", result)
}

func TestWelcomeGreeting_LongUsername(t *testing.T) {
	result := tui.WelcomeGreeting("areallylongusernameover20chars")
	assert.Equal(t, "Welcome back!", result)
}

// renderLogo

func TestRenderLogo_NotEmpty(t *testing.T) {
	result := tui.RenderLogo()
	assert.NotEmpty(t, result)
}

func TestRenderLogo_ConsistentWidth(t *testing.T) {
	result := tui.RenderLogo()
	lines := strings.Split(result, "\n")
	assert.True(t, len(lines) >= 3, "logo should have at least 3 lines")
	widths := make(map[int]bool)
	for _, line := range lines {
		if line == "" {
			continue
		}
		widths[len([]rune(line))] = true
	}
	assert.True(t, len(widths) <= 1, "all logo lines should have consistent rune width, got widths: %v", widths)
}

// RenderWelcome

func newWelcomeData(width int) tui.WelcomeData {
	return tui.WelcomeData{
		AppName:     "AQL",
		Version:     "0.1.0",
		ProjectPath: "/Users/amer/Code/github.com/amer/aql",
		ModelName:   "claude-sonnet-4-20250514",
		Username:    "amer",
		Width:       width,
	}
}

func TestRenderWelcome_ContainsGreeting(t *testing.T) {
	result := tui.RenderWelcome(newWelcomeData(60))
	assert.Contains(t, result, "Welcome back amer!")
}

func TestRenderWelcome_ContainsPath(t *testing.T) {
	result := tui.RenderWelcome(newWelcomeData(60))
	assert.Contains(t, result, "amer/aql")
}

func TestRenderWelcome_ContainsModel(t *testing.T) {
	result := tui.RenderWelcome(newWelcomeData(60))
	assert.Contains(t, result, "claude-sonnet-4-20250514")
}

func TestRenderWelcome_IsCompact(t *testing.T) {
	result := tui.RenderWelcome(newWelcomeData(80))
	lines := strings.Split(result, "\n")
	assert.Equal(t, 2, len(lines), "compact welcome should be exactly 2 lines")
}

func TestRenderWelcome_NoBorder(t *testing.T) {
	result := tui.RenderWelcome(newWelcomeData(60))
	assert.NotContains(t, result, "╭")
	assert.NotContains(t, result, "╰")
}
