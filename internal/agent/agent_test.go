package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amer/aql/internal/agent"
	"github.com/amer/aql/internal/agent/tools"
	"github.com/amer/aql/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockChatClient is a minimal ChatClient that does nothing — used for tests
// that only exercise agent construction and prompt building, not API calls.
type mockChatClient struct{}

func (m *mockChatClient) StreamMessage(_ context.Context, _ domain.ChatParams, _ func(string)) (*domain.ChatResponse, error) {
	return &domain.ChatResponse{StopReason: "end_turn"}, nil
}

func (m *mockChatClient) SendMessage(_ context.Context, _ domain.ChatParams) (*domain.ChatResponse, error) {
	return &domain.ChatResponse{StopReason: "end_turn"}, nil
}

func testConfig() agent.Config {
	return agent.Config{
		Name:         "coder",
		Role:         "Write clean Go code",
		SystemPrompt: "You are a senior Go developer.",
	}
}

func TestNewAgent(t *testing.T) {
	dir := t.TempDir()
	a, err := agent.New(testConfig(), dir, agent.WithChatClient(&mockChatClient{}))
	require.NoError(t, err)
	assert.Equal(t, "coder", a.Name())
}

func TestBuildSystemPrompt(t *testing.T) {
	prompt := agent.BuildSystemPrompt(testConfig(), "# Project Rules\n- Use TDD\n", "/tmp")

	assert.Contains(t, prompt, "You are a senior Go developer.")
	assert.Contains(t, prompt, "Write clean Go code")
	assert.Contains(t, prompt, "# Project Rules")
	assert.Contains(t, prompt, "Use TDD")
}

func TestBuildSystemPromptContainsEnvInfo(t *testing.T) {
	prompt := agent.BuildSystemPrompt(testConfig(), "", "/tmp")

	assert.Contains(t, prompt, "# Environment")
	assert.Contains(t, prompt, "Date:")
	assert.Contains(t, prompt, "Platform:")
}

func TestToolDescriptionsPrompt(t *testing.T) {
	desc := agent.ToolDescriptionsPrompt()
	assert.Contains(t, desc, "read_file")
	assert.Contains(t, desc, "write_file")
	assert.Contains(t, desc, "bash")
	assert.Contains(t, desc, "edit")
	assert.Contains(t, desc, "glob")
	assert.Contains(t, desc, "grep")
	assert.Contains(t, desc, "ask_user")
}

func TestToolDescriptionsPrompt_MatchesToolDefs(t *testing.T) {
	desc := agent.ToolDescriptionsPrompt()
	for _, td := range tools.Definitions() {
		assert.Contains(t, desc, td.Name, "tool %q missing from descriptions prompt", td.Name)
	}
}

func TestBuildSystemPromptNoClaudeMD(t *testing.T) {
	prompt := agent.BuildSystemPrompt(testConfig(), "", "/tmp")

	assert.Contains(t, prompt, "You are a senior Go developer.")
	assert.NotContains(t, prompt, "Project Rules")
}

func TestClaudeMDHotReload(t *testing.T) {
	dir := t.TempDir()

	// Start without CLAUDE.md
	a, err := agent.New(testConfig(), dir, agent.WithChatClient(&mockChatClient{}))
	require.NoError(t, err)
	assert.NotContains(t, a.SystemPrompt(), "hot-reload-test")

	// Write CLAUDE.md
	mdPath := filepath.Join(dir, "CLAUDE.md")
	require.NoError(t, os.WriteFile(mdPath, []byte("# hot-reload-test"), 0644))

	// Force a refresh (normally called by buildChatParams)
	a.RefreshClaudeMD()
	assert.Contains(t, a.SystemPrompt(), "hot-reload-test")

	// Update the file
	time.Sleep(10 * time.Millisecond) // ensure mtime differs
	require.NoError(t, os.WriteFile(mdPath, []byte("# updated-content"), 0644))

	a.RefreshClaudeMD()
	assert.Contains(t, a.SystemPrompt(), "updated-content")
	assert.NotContains(t, a.SystemPrompt(), "hot-reload-test")
}

func TestClearHistory_RemovesAllMessages(t *testing.T) {
	dir := t.TempDir()
	a, err := agent.New(testConfig(), dir, agent.WithChatClient(&mockChatClient{}))
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
	a, err := agent.New(testConfig(), dir, agent.WithChatClient(&mockChatClient{}))
	require.NoError(t, err)

	// Should not panic on empty history
	a.ClearHistory()
	assert.Equal(t, 0, a.HistoryLen())
}

func TestAgentWithClaudeMD(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Rules\n- TDD always\n"), 0644))

	a, err := agent.New(testConfig(), dir, agent.WithChatClient(&mockChatClient{}))
	require.NoError(t, err)

	prompt := a.SystemPrompt()
	assert.Contains(t, prompt, "TDD always")
}
