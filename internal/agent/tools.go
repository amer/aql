package agent

import (
	"context"
	"encoding/json"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/anthropics/anthropic-sdk-go"
)

// Type aliases — canonical definitions live in the tools sub-package.
type (
	ToolDef      = tools.ToolDef
	UserQuestion = tools.UserQuestion
)

// ToolDefinitions returns the set of tools available to agents.
func ToolDefinitions() []ToolDef {
	return tools.Definitions()
}

// ToAPITools converts tool definitions to the Anthropic API format.
func ToAPITools(defs []ToolDef) []anthropic.ToolUnionParam {
	return tools.ToAPITools(defs)
}

// DefaultToolExecutor returns a ToolExecutorFn that dispatches to the
// built-in tool implementations, using askFn for the ask_user tool.
func DefaultToolExecutor(askFn AskUserFn) ToolExecutorFn {
	return tools.DefaultExecutor(askFn)
}

// ExecuteTool runs a tool by name using the default executor with no ask_user support.
// Primarily useful for tests that exercise individual tools.
func ExecuteTool(ctx context.Context, workDir string, name string, input json.RawMessage) (string, error) {
	return tools.Execute(ctx, workDir, name, input)
}
