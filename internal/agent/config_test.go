package agent_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amer/aql/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testConfigYAML = `name: coder
role: "Write clean, tested Go code following TDD"
system_prompt: |
  You are a senior Go developer.
  Always write failing tests first.
tools:
  - read_file
  - write_file
  - bash
memory:
  private: true
  shared_access:
    - project
    - architecture
events:
  publishes:
    - code_written
    - test_written
  subscribes:
    - review_feedback
`

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "coder.yaml")
	require.NoError(t, os.WriteFile(path, []byte(testConfigYAML), 0644))

	cfg, err := agent.LoadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, "coder", cfg.Name)
	assert.Equal(t, "Write clean, tested Go code following TDD", cfg.Role)
	assert.Contains(t, cfg.SystemPrompt, "senior Go developer")
	assert.Equal(t, []string{"read_file", "write_file", "bash"}, cfg.Tools)
	assert.True(t, cfg.Memory.Private)
	assert.Equal(t, []string{"project", "architecture"}, cfg.Memory.SharedAccess)
	assert.Equal(t, []string{"code_written", "test_written"}, cfg.Events.Publishes)
	assert.Equal(t, []string{"review_feedback"}, cfg.Events.Subscribes)
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := agent.LoadConfig("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte(":::invalid"), 0644))

	_, err := agent.LoadConfig(path)
	assert.Error(t, err)
}

func TestParseConfig(t *testing.T) {
	cfg, err := agent.ParseConfig([]byte(testConfigYAML))
	require.NoError(t, err)
	assert.Equal(t, "coder", cfg.Name)
	assert.Len(t, cfg.Tools, 3)
}

func TestParseConfigEmpty(t *testing.T) {
	cfg, err := agent.ParseConfig([]byte(""))
	require.NoError(t, err)
	assert.Equal(t, "", cfg.Name)
}
