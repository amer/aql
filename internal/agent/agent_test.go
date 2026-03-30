package agent_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amer/aql/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig() agent.Config {
	return agent.Config{
		Name:         "coder",
		Role:         "Write clean Go code",
		SystemPrompt: "You are a senior Go developer.",
		Tools:        []string{"read_file", "write_file"},
		Memory: agent.MemoryConfig{
			Private:      true,
			SharedAccess: []string{"project"},
		},
	}
}

func TestNewAgent(t *testing.T) {
	dir := t.TempDir()
	a, err := agent.New(testConfig(), dir)
	require.NoError(t, err)
	assert.Equal(t, "coder", a.Name())
}

func TestBuildSystemPrompt(t *testing.T) {
	prompt := agent.BuildSystemPrompt(testConfig(), "# Project Rules\n- Use TDD\n")

	assert.Contains(t, prompt, "You are a senior Go developer.")
	assert.Contains(t, prompt, "Write clean Go code")
	assert.Contains(t, prompt, "# Project Rules")
	assert.Contains(t, prompt, "Use TDD")
}

func TestBuildSystemPromptNoClaudeMD(t *testing.T) {
	prompt := agent.BuildSystemPrompt(testConfig(), "")

	assert.Contains(t, prompt, "You are a senior Go developer.")
	assert.NotContains(t, prompt, "Project Rules")
}

func TestBuildSystemPromptWithMemoryContext(t *testing.T) {
	memories := []string{
		"Previously implemented auth module using JWT",
		"Team prefers table-driven tests",
	}

	prompt := agent.BuildSystemPromptWithMemories(testConfig(), "", memories)

	assert.Contains(t, prompt, "JWT")
	assert.Contains(t, prompt, "table-driven tests")
}

func TestClearHistory_RemovesAllMessages(t *testing.T) {
	dir := t.TempDir()
	a, err := agent.New(testConfig(), dir)
	require.NoError(t, err)

	assert.Equal(t, 0, a.HistoryLen(), "new agent starts with empty history")

	// Simulate a conversation: add messages via AppendHistory
	a.AppendUserMessage("write auth tests")
	a.AppendAssistantMessage("I'll write the tests now.")
	a.AppendUserMessage("looks good, add error cases too")
	assert.Equal(t, 3, a.HistoryLen(), "history should have 3 messages after conversation")

	a.ClearHistory()
	assert.Equal(t, 0, a.HistoryLen(), "history should be empty after ClearHistory")
}

func TestClearHistory_SafeOnEmptyHistory(t *testing.T) {
	dir := t.TempDir()
	a, err := agent.New(testConfig(), dir)
	require.NoError(t, err)

	// Should not panic on empty history
	a.ClearHistory()
	assert.Equal(t, 0, a.HistoryLen())
}

func TestAgentWithClaudeMD(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Rules\n- TDD always\n"), 0644))

	a, err := agent.New(testConfig(), dir)
	require.NoError(t, err)

	prompt := a.SystemPrompt()
	assert.Contains(t, prompt, "TDD always")
}
