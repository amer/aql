package tui_test

import (
	"regexp"
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func stripANSIStatus(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

func TestRenderStatusBar(t *testing.T) {
	result := tui.RenderStatusBar("claude-sonnet-4", 1500, 60)
	plain := stripANSIStatus(result)
	assert.Contains(t, plain, "agents on")
	assert.Contains(t, plain, "auto-compact")
}

func TestRenderStatusBarTokenPercentage(t *testing.T) {
	// 100k out of 200k max = 50%
	result := tui.RenderStatusBar("claude-sonnet-4", 100000, 60)
	plain := stripANSIStatus(result)
	assert.Contains(t, plain, "% until auto-compact")
}

func TestRenderStatusBarSmall(t *testing.T) {
	result := tui.RenderStatusBar("claude-sonnet-4", 42, 60)
	plain := stripANSIStatus(result)
	assert.Contains(t, plain, "auto-compact")
}
