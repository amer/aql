package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amer/aql/internal/agent"
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

// buildPrompt assembles the system prompt via the production path
// (BuildPromptParts + JoinPromptParts) so these tests exercise what agents
// actually send, not a convenience wrapper.
func buildPrompt(cfg agent.Config, claudeMD, workDir string) string {
	return agent.JoinPromptParts(agent.BuildPromptParts(cfg, claudeMD, workDir))
}

func TestBuildSystemPrompt(t *testing.T) {
	prompt := buildPrompt(testConfig(), "# Project Rules\n- Use TDD\n", "/tmp")

	assert.Contains(t, prompt, "You are a senior Go developer.")
	assert.Contains(t, prompt, "Write clean Go code")
	assert.Contains(t, prompt, "# Project Rules")
	assert.Contains(t, prompt, "Use TDD")
}

func TestBuildSystemPromptContainsEnvInfo(t *testing.T) {
	prompt := buildPrompt(testConfig(), "", "/tmp")

	assert.Contains(t, prompt, "# Environment")
	assert.Contains(t, prompt, "Date:")
	assert.Contains(t, prompt, "Platform:")
}

func TestBuildSystemPromptNoClaudeMD(t *testing.T) {
	prompt := buildPrompt(testConfig(), "", "/tmp")

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
	a.ApplyHistory(domain.NewUserMessage("write auth tests"))
	a.ApplyHistory(domain.NewAssistantMessage("I'll write the tests now."))
	a.ApplyHistory(domain.NewUserMessage("looks good, add error cases too"))
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

func TestBuildPromptParts_ReturnsNamedParts(t *testing.T) {
	parts := agent.BuildPromptParts(testConfig(), "# Rules\n- TDD\n", "/tmp")

	names := make([]string, len(parts))
	for i, p := range parts {
		names[i] = p.Name
	}

	assert.Contains(t, names, "role")
	assert.Contains(t, names, "system")
	assert.NotContains(t, names, "tools", "tool descriptions should not be in system prompt — they are sent as structured API tools")
	assert.Contains(t, names, "environment")
	assert.Contains(t, names, "project-context")

	for _, p := range parts {
		assert.NotEmpty(t, p.Content, "part %q should have content", p.Name)
	}
}

func TestBuildPromptParts_OmitsEmptyParts(t *testing.T) {
	parts := agent.BuildPromptParts(testConfig(), "", "/tmp")

	names := make([]string, len(parts))
	for i, p := range parts {
		names[i] = p.Name
	}

	assert.NotContains(t, names, "project-context")
}

func TestJoinPromptParts(t *testing.T) {
	parts := []agent.PromptPart{
		{Name: "a", Content: "hello"},
		{Name: "b", Content: "world"},
	}

	result := agent.JoinPromptParts(parts)
	assert.Equal(t, "hello\n\nworld", result)
}

func TestAgentWithClaudeMD(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Rules\n- TDD always\n"), 0644))

	a, err := agent.New(testConfig(), dir, agent.WithChatClient(&mockChatClient{}))
	require.NoError(t, err)

	prompt := a.SystemPrompt()
	assert.Contains(t, prompt, "TDD always")
}
