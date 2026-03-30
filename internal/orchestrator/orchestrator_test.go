package orchestrator_test

import (
	"context"
	"testing"

	"github.com/amer/aql/internal/agent"
	"github.com/amer/aql/internal/events"
	"github.com/amer/aql/internal/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeAgent(t *testing.T, name string) *agent.Agent {
	t.Helper()
	dir := t.TempDir()
	a, err := agent.New(agent.Config{
		Name:         name,
		Role:         name + " role",
		SystemPrompt: "You are " + name,
	}, dir)
	require.NoError(t, err)
	return a
}

func TestNewOrchestrator(t *testing.T) {
	wf := orchestrator.Workflow{
		Name:   "test-workflow",
		Agents: []string{"coder", "reviewer"},
		Execution: orchestrator.Execution{
			Mode: "parallel",
			Pairs: []orchestrator.Pair{
				{Agents: []string{"coder", "reviewer"}, Relationship: "pair"},
			},
		},
	}

	orch := orchestrator.New(wf)
	assert.NotNil(t, orch)
	assert.Equal(t, "test-workflow", orch.WorkflowName())
}

func TestOrchestratorRegisterAgents(t *testing.T) {
	wf := orchestrator.Workflow{Name: "test"}
	orch := orchestrator.New(wf)

	orch.RegisterAgent(makeAgent(t, "coder"))
	orch.RegisterAgent(makeAgent(t, "reviewer"))

	agents := orch.Agents()
	assert.Len(t, agents, 2)
}

func TestOrchestratorEventBus(t *testing.T) {
	wf := orchestrator.Workflow{Name: "test"}
	orch := orchestrator.New(wf)

	received := make(chan events.Event, 1)
	orch.Bus().Subscribe("test_event", func(e events.Event) {
		received <- e
	})

	orch.Bus().Publish(events.Event{
		Type:    "test_event",
		AgentID: "coder",
		Payload: "hello",
	})

	e := <-received
	assert.Equal(t, "hello", e.Payload)
}

func TestOrchestratorGetAgent(t *testing.T) {
	wf := orchestrator.Workflow{Name: "test"}
	orch := orchestrator.New(wf)
	orch.RegisterAgent(makeAgent(t, "coder"))

	a, ok := orch.GetAgent("coder")
	assert.True(t, ok)
	assert.Equal(t, "coder", a.Name())

	_, ok = orch.GetAgent("nonexistent")
	assert.False(t, ok)
}

func TestOrchestratorStatus(t *testing.T) {
	wf := orchestrator.Workflow{Name: "test"}
	orch := orchestrator.New(wf)

	assert.Equal(t, orchestrator.StatusIdle, orch.Status())
}

func TestOrchestratorStartAndStop(t *testing.T) {
	wf := orchestrator.Workflow{
		Name:   "test",
		Agents: []string{"coder"},
		Execution: orchestrator.Execution{
			Mode: "parallel",
		},
	}
	orch := orchestrator.New(wf)
	orch.RegisterAgent(makeAgent(t, "coder"))

	ctx, cancel := context.WithCancel(context.Background())
	errCh := orch.Start(ctx)

	assert.Equal(t, orchestrator.StatusRunning, orch.Status())

	cancel()
	err := <-errCh
	assert.NoError(t, err)
	assert.Equal(t, orchestrator.StatusStopped, orch.Status())
}
