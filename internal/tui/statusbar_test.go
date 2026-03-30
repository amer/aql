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

func TestRenderStatusBar_ShowsModel(t *testing.T) {
	result := tui.RenderStatusBar("claude-sonnet-4", 1500, 60)
	plain := stripANSIStatus(result)
	assert.Contains(t, plain, "claude-sonnet-4")
}

func TestRenderStatusBar_ShowsTokenCount(t *testing.T) {
	result := tui.RenderStatusBar("claude-sonnet-4", 1500, 60)
	plain := stripANSIStatus(result)
	assert.Contains(t, plain, "1.5k tokens")
}

func TestRenderStatusBar_LargeTokenCount(t *testing.T) {
	result := tui.RenderStatusBar("claude-sonnet-4", 100000, 60)
	plain := stripANSIStatus(result)
	assert.Contains(t, plain, "100.0k tokens")
}

func TestRenderStatusBar_ZeroTokens(t *testing.T) {
	result := tui.RenderStatusBar("claude-sonnet-4", 0, 60)
	plain := stripANSIStatus(result)
	assert.Contains(t, plain, "0 tokens")
}
