package agent_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/amer/aql/internal/agent"
	"github.com/amer/aql/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeChatClient returns a canned response for testing.
type fakeChatClient struct {
	response *domain.ChatResponse
}

func (f *fakeChatClient) StreamMessage(_ context.Context, _ domain.ChatParams, onText func(string)) (*domain.ChatResponse, error) {
	for _, t := range f.response.TextParts {
		onText(t)
	}
	return f.response, nil
}

func (f *fakeChatClient) SendMessage(_ context.Context, _ domain.ChatParams) (*domain.ChatResponse, error) {
	return f.response, nil
}

func TestSpawner_ReturnsChildAgentText(t *testing.T) {
	client := &fakeChatClient{
		response: &domain.ChatResponse{
			TextParts:  []string{"found 3 matching files"},
			StopReason: "end_turn",
		},
	}
	spawner := agent.NewSpawner(client, spawnerTestConfig(), t.TempDir())

	result, err := spawner.Spawn(context.Background(), "find Go files")

	require.NoError(t, err)
	assert.Equal(t, "found 3 matching files", result)
}

func TestSpawner_ConcatenatesMultipleTextParts(t *testing.T) {
	client := &fakeChatClient{
		response: &domain.ChatResponse{
			TextParts:  []string{"part one", " part two"},
			StopReason: "end_turn",
		},
	}
	spawner := agent.NewSpawner(client, spawnerTestConfig(), t.TempDir())

	result, err := spawner.Spawn(context.Background(), "summarize")

	require.NoError(t, err)
	assert.Equal(t, "part one part two", result)
}

func TestSpawner_DepthLimit(t *testing.T) {
	client := &fakeChatClient{
		response: &domain.ChatResponse{
			TextParts:  []string{"ok"},
			StopReason: "end_turn",
		},
	}
	spawner := agent.NewSpawner(client, spawnerTestConfig(), t.TempDir(), agent.WithMaxDepth(1))

	// First level should work
	_, err := spawner.Spawn(context.Background(), "task 1")
	require.NoError(t, err)
}

func TestSpawner_DepthZeroRejects(t *testing.T) {
	client := &fakeChatClient{
		response: &domain.ChatResponse{
			TextParts:  []string{"ok"},
			StopReason: "end_turn",
		},
	}
	spawner := agent.NewSpawner(client, spawnerTestConfig(), t.TempDir(), agent.WithMaxDepth(0))

	_, err := spawner.Spawn(context.Background(), "task 1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "depth limit")
}

func TestSpawner_ToolUseLoop(t *testing.T) {
	// Simulate a child agent that uses a tool then finishes
	callCount := 0
	client := &toolUsingClient{
		responses: []*domain.ChatResponse{
			{
				TextParts: nil,
				ToolUses: []domain.ChatToolUse{
					{ID: "tool_1", Name: "bash", Input: `{"command":"echo hi"}`},
				},
				StopReason: "tool_use",
			},
			{
				TextParts:  []string{"tool output was: hi"},
				StopReason: "end_turn",
			},
		},
		callCount: &callCount,
	}
	spawner := agent.NewSpawner(client, spawnerTestConfig(), t.TempDir())

	result, err := spawner.Spawn(context.Background(), "run echo")

	require.NoError(t, err)
	assert.Contains(t, result, "tool output was: hi")
}

// toolUsingClient returns different responses on successive calls.
type toolUsingClient struct {
	responses []*domain.ChatResponse
	callCount *int
}

func (t *toolUsingClient) StreamMessage(_ context.Context, params domain.ChatParams, onText func(string)) (*domain.ChatResponse, error) {
	idx := *t.callCount
	if idx >= len(t.responses) {
		idx = len(t.responses) - 1
	}
	*t.callCount++

	// Check if this call includes tool results (means it's a follow-up after tool use)
	hasToolResult := false
	for _, msg := range params.Messages {
		for _, cb := range msg.Content {
			if cb.ToolResult != nil {
				hasToolResult = true
			}
		}
	}
	_ = hasToolResult

	resp := t.responses[idx]
	for _, part := range resp.TextParts {
		onText(part)
	}
	return resp, nil
}

func (t *toolUsingClient) SendMessage(_ context.Context, _ domain.ChatParams) (*domain.ChatResponse, error) {
	return t.responses[0], nil
}

func spawnerTestConfig() agent.Config {
	return agent.Config{
		Name:         "test-agent",
		Role:         "assistant",
		SystemPrompt: "You are a test assistant.",
		Model:        "claude-haiku-4-5",
	}
}

// Verify the spawner implements the tools.AgentSpawner interface
func TestSpawner_ImplementsInterface(t *testing.T) {
	client := &fakeChatClient{
		response: &domain.ChatResponse{
			TextParts:  []string{"ok"},
			StopReason: "end_turn",
		},
	}
	spawner := agent.NewSpawner(client, spawnerTestConfig(), t.TempDir())

	// This assignment verifies interface compliance at compile time
	var _ interface {
		Spawn(ctx context.Context, prompt string) (string, error)
	} = spawner

	// Also test round-trip through tools executor
	exec := agent.NewToolExecutor(spawnerTestConfig(), client, t.TempDir())
	result, err := exec(context.Background(), t.TempDir(), "agent",
		json.RawMessage(`{"prompt":"hello","description":"test"}`))
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}
