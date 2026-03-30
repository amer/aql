package orchestrator_test

import (
	"testing"

	"github.com/amer/aql/internal/agent"
	"github.com/amer/aql/internal/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAgent(t *testing.T, name string) *agent.Agent {
	t.Helper()
	dir := t.TempDir()
	a, err := agent.New(agent.Config{
		Name:         name,
		Role:         "test agent",
		SystemPrompt: "You are a test agent.",
	}, dir)
	require.NoError(t, err)
	return a
}

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := orchestrator.NewRegistry()
	a := testAgent(t, "coder")

	reg.Register(a)

	got, ok := reg.Get("coder")
	assert.True(t, ok)
	assert.Equal(t, "coder", got.Name())
}

func TestRegistryGetNotFound(t *testing.T) {
	reg := orchestrator.NewRegistry()

	_, ok := reg.Get("nonexistent")
	assert.False(t, ok)
}

func TestRegistryList(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register(testAgent(t, "coder"))
	reg.Register(testAgent(t, "reviewer"))

	agents := reg.List()
	assert.Len(t, agents, 2)
}

func TestRegistryOverwrite(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register(testAgent(t, "coder"))
	reg.Register(testAgent(t, "coder"))

	agents := reg.List()
	assert.Len(t, agents, 1)
}
