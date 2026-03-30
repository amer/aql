package agent

import "github.com/anthropics/anthropic-sdk-go"

// ResolveModel maps a model string to an anthropic.Model.
// Supports shortcuts ("haiku", "sonnet", "opus") and full model IDs.
// Defaults to Haiku if empty.
func ResolveModel(model string) anthropic.Model {
	switch model {
	case "", "haiku":
		return anthropic.ModelClaudeHaiku4_5
	case "sonnet":
		return anthropic.ModelClaudeSonnet4_5
	case "opus":
		return anthropic.ModelClaudeOpus4_5
	default:
		return anthropic.Model(model)
	}
}
