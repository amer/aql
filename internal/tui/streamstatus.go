package tui

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - StreamPhase enum, PhaseArrow, StreamStatus struct,
//     FormatStreamStatus, RenderStreamingIndicator,
//     RenderCompletionIndicator, EstimateTokens, formatDuration.
//
// MUST NOT GO HERE:
//   - Stream state management (handlers.go), actual API interaction.
// ──────────────────────────────────────────────────────────────────

import (
	"fmt"
	"time"
)

// StreamPhase represents the current phase of an API streaming interaction.
type StreamPhase int

const (
	// PhaseResponding means tokens are arriving from the API (output).
	PhaseResponding StreamPhase = iota
	// PhaseRequesting means tokens are being sent to the API (input).
	PhaseRequesting
	// PhaseThinking means the model is thinking (output).
	PhaseThinking
	// PhaseToolUse means the model is calling a tool (output).
	PhaseToolUse
)

// PhaseArrow returns the arrow direction for a streaming phase.
// ↑ for requesting (input to API), ↓ for all other phases (output from API).
func PhaseArrow(p StreamPhase) string {
	if p == PhaseRequesting {
		return "↑"
	}
	return "↓"
}

// StreamStatus holds the live stats for a streaming response.
type StreamStatus struct {
	Elapsed      time.Duration
	Tokens       int
	ThinkingTime time.Duration
	Phase        StreamPhase
}

// FormatStreamStatus formats streaming stats like Claude Code:
// Requesting: "3s · ↑ 0 tokens"
// Responding: "5s · ↓ 120 tokens" or "33s · ↓ 598 tokens · thought for 1s"
func FormatStreamStatus(s StreamStatus) string {
	elapsed := formatDuration(s.Elapsed)

	result := elapsed + " · " + PhaseArrow(s.Phase) + " " + FormatTokenCount(s.Tokens) + " tokens"

	if s.ThinkingTime > 0 {
		result += " · thought for " + formatDuration(s.ThinkingTime)
	}

	return result
}

// RenderStreamingIndicator renders the full Claude Code-style streaming line:
// ⠋ Composing… (33s · ↓ 598 tokens · thought for 1s)
// The spinner frame animates using the given spinner type.
func RenderStreamingIndicator(frame int, agentName string, s StreamStatus, st SpinnerType) string {
	symbol := AccentStyle.Render(SpinnerFrameAt(st, frame))
	label := AccentStyle.Render("Composing…")
	stats := MutedStyle.Render("(" + FormatStreamStatus(s) + ")")
	return symbol + " " + label + " " + stats
}

// RenderCompletionIndicator renders the post-response summary line:
// ✻ Crunched for 8m35s
func RenderCompletionIndicator(elapsed time.Duration) string {
	symbol := AccentStyle.Render("✻")
	label := MutedStyle.Render("Crunched for " + formatDuration(elapsed))
	return symbol + " " + label
}

// EstimateTokens estimates token count from character count (~4 chars per token).
func EstimateTokens(chars int) int {
	if chars <= 0 {
		return 0
	}
	return chars / 4
}

// formatDuration formats a duration compactly: "5s", "1m30s", "2m15s".
func formatDuration(d time.Duration) string {
	d = d.Truncate(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if s == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dm%ds", m, s)
}
