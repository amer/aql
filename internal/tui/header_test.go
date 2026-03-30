package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func TestRenderWelcome_ShowsAllInfo(t *testing.T) {
	result := tui.RenderWelcome(tui.WelcomeData{
		AppName:     "AQL",
		Version:     "0.1.0",
		ProjectPath: "/home/user/project",
		ModelName:   "claude-sonnet-4",
		Username:    "testuser",
		Width:       80,
	})
	assert.Contains(t, result, "/home/user/project")
	assert.Contains(t, result, "claude-sonnet-4")
	assert.Contains(t, result, "Welcome back testuser!")
}
