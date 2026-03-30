package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func TestRenderHeader(t *testing.T) {
	result := tui.RenderHeader("/home/user/project", "claude-sonnet-4", 80)
	assert.Contains(t, result, "AQL")
	assert.Contains(t, result, "/home/user/project")
	assert.Contains(t, result, "claude-sonnet-4")
}
