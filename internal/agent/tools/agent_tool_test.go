package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSpawner implements tools.AgentSpawner for testing.
type fakeSpawner struct {
	result string
	err    error
}

func (f *fakeSpawner) Spawn(ctx context.Context, prompt string) (string, error) {
	return f.result, f.err
}

func TestAgentTool_ReturnsSubAgentResult(t *testing.T) {
	spawner := &fakeSpawner{result: "sub-agent found 3 files"}
	exec := tools.NewExecutor(tools.WithAgentSpawner(spawner))

	result, err := exec(context.Background(), "/tmp", "agent",
		json.RawMessage(`{"prompt":"find all Go files","description":"search files"}`))

	require.NoError(t, err)
	assert.Equal(t, "sub-agent found 3 files", result)
}

func TestAgentTool_MissingPrompt(t *testing.T) {
	spawner := &fakeSpawner{result: "ok"}
	exec := tools.NewExecutor(tools.WithAgentSpawner(spawner))

	result, err := exec(context.Background(), "/tmp", "agent",
		json.RawMessage(`{"description":"search"}`))

	require.NoError(t, err)
	assert.Contains(t, result, "prompt is required")
}

func TestAgentTool_SpawnerError(t *testing.T) {
	spawner := &fakeSpawner{err: errors.New("depth limit exceeded")}
	exec := tools.NewExecutor(tools.WithAgentSpawner(spawner))

	result, err := exec(context.Background(), "/tmp", "agent",
		json.RawMessage(`{"prompt":"do something","description":"task"}`))

	require.NoError(t, err)
	assert.Contains(t, result, "depth limit exceeded")
}

func TestAgentTool_NoSpawnerConfigured(t *testing.T) {
	exec := tools.NewExecutor() // no spawner

	result, err := exec(context.Background(), "/tmp", "agent",
		json.RawMessage(`{"prompt":"hello","description":"test"}`))

	require.NoError(t, err)
	assert.Contains(t, result, "sub-agents are not available")
}

func TestAgentTool_ContextCanceled(t *testing.T) {
	spawner := &fakeSpawner{err: context.Canceled}
	exec := tools.NewExecutor(tools.WithAgentSpawner(spawner))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := exec(ctx, "/tmp", "agent",
		json.RawMessage(`{"prompt":"hello","description":"test"}`))

	require.NoError(t, err)
	assert.Contains(t, result, "canceled")
}

func TestAgentToolInDefinitions(t *testing.T) {
	defs := tools.Definitions()
	found := false
	for _, d := range defs {
		if d.Name == "agent" {
			found = true
			break
		}
	}
	assert.True(t, found, "agent should be in Definitions()")
}
