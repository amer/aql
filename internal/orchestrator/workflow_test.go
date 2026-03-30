package orchestrator_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amer/aql/internal/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testWorkflowYAML = `name: pair-programming
agents:
  - coder
  - reviewer
  - doc-writer

execution:
  mode: parallel
  pairs:
    - agents: [coder, reviewer]
      relationship: pair
    - agents: [coder, doc-writer]
      relationship: follow
    - agents: [architect-checker]
      relationship: audit
`

func TestParseWorkflow(t *testing.T) {
	wf, err := orchestrator.ParseWorkflow([]byte(testWorkflowYAML))
	require.NoError(t, err)

	assert.Equal(t, "pair-programming", wf.Name)
	assert.Equal(t, []string{"coder", "reviewer", "doc-writer"}, wf.Agents)
	assert.Equal(t, "parallel", wf.Execution.Mode)
	require.Len(t, wf.Execution.Pairs, 3)
	assert.Equal(t, []string{"coder", "reviewer"}, wf.Execution.Pairs[0].Agents)
	assert.Equal(t, "pair", wf.Execution.Pairs[0].Relationship)
	assert.Equal(t, "audit", wf.Execution.Pairs[2].Relationship)
}

func TestLoadWorkflow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.yaml")
	require.NoError(t, os.WriteFile(path, []byte(testWorkflowYAML), 0644))

	wf, err := orchestrator.LoadWorkflow(path)
	require.NoError(t, err)
	assert.Equal(t, "pair-programming", wf.Name)
}

func TestLoadWorkflowNotFound(t *testing.T) {
	_, err := orchestrator.LoadWorkflow("/nonexistent.yaml")
	assert.Error(t, err)
}

func TestParseWorkflowInvalid(t *testing.T) {
	_, err := orchestrator.ParseWorkflow([]byte(":::bad"))
	assert.Error(t, err)
}

func TestWorkflowAgentsInPairs(t *testing.T) {
	wf, err := orchestrator.ParseWorkflow([]byte(testWorkflowYAML))
	require.NoError(t, err)

	agents := wf.AllAgentsInPairs()
	assert.Contains(t, agents, "coder")
	assert.Contains(t, agents, "reviewer")
	assert.Contains(t, agents, "doc-writer")
	assert.Contains(t, agents, "architect-checker")
}

func TestWorkflowPairsForAgent(t *testing.T) {
	wf, err := orchestrator.ParseWorkflow([]byte(testWorkflowYAML))
	require.NoError(t, err)

	pairs := wf.PairsForAgent("coder")
	assert.Len(t, pairs, 2)

	pairs = wf.PairsForAgent("architect-checker")
	assert.Len(t, pairs, 1)

	pairs = wf.PairsForAgent("nonexistent")
	assert.Len(t, pairs, 0)
}
