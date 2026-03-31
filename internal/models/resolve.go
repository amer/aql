package models

import (
	"fmt"
	"log/slog"
	"strings"
)

// Well-known model IDs — plain strings instead of SDK constants
// to keep this package SDK-free.
const (
	ModelSonnet = "claude-sonnet-4-6"
	ModelOpus   = "claude-opus-4-6"
	ModelHaiku  = "claude-haiku-4-5"
)

// ResolveModel maps a model string to a concrete model ID.
// Supports shortcuts ("haiku", "sonnet", "opus") and full model IDs.
// Defaults to Sonnet if empty. Rejects obviously invalid values
// (slash commands) by falling back to the default.
func ResolveModel(model string) string {
	switch model {
	case "", "sonnet":
		return ModelSonnet
	case "opus":
		return ModelOpus
	case "haiku":
		return ModelHaiku
	default:
		if err := ValidateModelID(model); err != nil {
			slog.Warn("invalid model ID, using default", "model", model, "error", err)
			return ModelSonnet
		}
		return model
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
