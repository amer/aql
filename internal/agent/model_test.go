package agent_test

import (
	"testing"

	"github.com/amer/aql/internal/agent"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
)

func TestResolveModelDefault(t *testing.T) {
	model := agent.ResolveModel("")
	assert.Equal(t, anthropic.ModelClaudeHaiku4_5, model)
}

func TestResolveModelExplicit(t *testing.T) {
	model := agent.ResolveModel("claude-sonnet-4-5")
	assert.Equal(t, anthropic.Model("claude-sonnet-4-5"), model)
}

func TestResolveModelShortcuts(t *testing.T) {
	assert.Equal(t, anthropic.ModelClaudeHaiku4_5, agent.ResolveModel("haiku"))
	assert.Equal(t, anthropic.ModelClaudeSonnet4_5, agent.ResolveModel("sonnet"))
	assert.Equal(t, anthropic.ModelClaudeOpus4_5, agent.ResolveModel("opus"))
}

func TestConfigModel(t *testing.T) {
	cfg := agent.Config{
		Name:  "test",
		Model: "haiku",
	}
	assert.Equal(t, "haiku", cfg.Model)
}
