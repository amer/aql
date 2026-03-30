package tui

import (
	"fmt"
	"time"
)

// StreamStatus holds the live stats for a streaming response.
type StreamStatus struct {
	Elapsed      time.Duration
	Tokens       int
	ThinkingTime time.Duration
}

// FormatStreamStatus formats streaming stats like Claude Code:
// "5s · ↓ 120 tokens" or "33s · ↓ 598 tokens · thought for 1s"
func FormatStreamStatus(s StreamStatus) string {
	elapsed := formatDuration(s.Elapsed)
	tokens := FormatTokenCount(s.Tokens) + " tokens"

	result := elapsed + " · ↓ " + tokens

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
