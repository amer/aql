package models

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// ResolveModel maps a model string to an anthropic.Model.
// Supports shortcuts ("haiku", "sonnet", "opus") and full model IDs.
// Defaults to Sonnet if empty. Rejects obviously invalid values
// (slash commands) by falling back to the default.
func ResolveModel(model string) anthropic.Model {
	switch model {
	case "", "sonnet":
		return anthropic.ModelClaudeSonnet4_6
	case "opus":
		return anthropic.ModelClaudeOpus4_6
	case "haiku":
		return anthropic.ModelClaudeHaiku4_5
	default:
		if err := ValidateModelID(model); err != nil {
			slog.Warn("invalid model ID, using default", "model", model, "error", err)
			return anthropic.ModelClaudeSonnet4_6
		}
		return anthropic.Model(model)
	}
}

// ValidateModelID checks that a model ID looks valid (not empty, not a slash command).
func ValidateModelID(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return fmt.Errorf("model ID must not be empty")
	}
	if strings.HasPrefix(trimmed, "/") {
		return fmt.Errorf("model ID %q looks like a slash command, not a model", id)
	}
	return nil
}
