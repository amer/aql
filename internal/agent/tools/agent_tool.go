package tools

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - AgentSpawner interface (consumer-side port)
//   - registerAgentTool — adds agent handler to registry
//   - execAgent — validates input and delegates to spawner
//
// MUST NOT GO HERE:
//   - Spawner implementation (that's internal/agent/spawner.go —
//     avoids circular import)
//   - Tool definitions (defs.go)
//
// Q: Why is AgentSpawner defined here, not in the agent package?
// A: To avoid circular imports. tools defines the interface (consumer),
//    agent implements it (producer).
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"encoding/json"
)

// AgentSpawner creates and runs a child agent with its own conversation context.
// The real implementation lives in the agent package to avoid circular imports.
type AgentSpawner interface {
	Spawn(ctx context.Context, prompt string) (string, error)
}

// registerAgentTool adds the agent tool handler to the registry.
func registerAgentTool(registry map[string]toolHandler, spawner AgentSpawner) {
	registry["agent"] = func(ctx context.Context, _ string, input json.RawMessage) (string, error) {
		return execAgent(ctx, input, spawner)
	}
}

func execAgent(ctx context.Context, input json.RawMessage, spawner AgentSpawner) (string, error) {
	if spawner == nil {
		return "sub-agents are not available in this session", nil
	}

	var params struct {
		Prompt      string `json:"prompt"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "invalid input: " + err.Error(), nil
	}
	if params.Prompt == "" {
		return "prompt is required", nil
	}

	result, err := spawner.Spawn(ctx, params.Prompt)
	if err != nil {
		return "sub-agent error: " + err.Error(), nil
	}
	return result, nil
}
