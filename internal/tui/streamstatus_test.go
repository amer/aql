package tui_test

import (
	"testing"
	"time"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func TestStreamStatus_Basic(t *testing.T) {
	s := tui.StreamStatus{
		Elapsed: 5 * time.Second,
		Tokens:  120,
	}
	result := tui.FormatStreamStatus(s)
	assert.Contains(t, result, "5s")
	assert.Contains(t, result, "120 tokens")
	assert.Contains(t, result, "↓")
}

func TestStreamStatus_WithThinking(t *testing.T) {
	s := tui.StreamStatus{
		Elapsed:      33 * time.Second,
		Tokens:       598,
		ThinkingTime: 1 * time.Second,
	}
	result := tui.FormatStreamStatus(s)
	assert.Contains(t, result, "33s")
	assert.Contains(t, result, "598 tokens")
	assert.Contains(t, result, "thought for 1s")
}

func TestStreamStatus_NoThinking(t *testing.T) {
	s := tui.StreamStatus{
		Elapsed: 10 * time.Second,
		Tokens:  250,
	}
	result := tui.FormatStreamStatus(s)
	assert.NotContains(t, result, "thought")
}

func TestStreamStatus_MinuteFormat(t *testing.T) {
	s := tui.StreamStatus{
		Elapsed: 2*time.Minute + 15*time.Second,
		Tokens:  5000,
	}
	result := tui.FormatStreamStatus(s)
	assert.Contains(t, result, "2m15s")
	assert.Contains(t, result, "5,000 tokens")
}

func TestStreamStatus_ZeroTokens(t *testing.T) {
	s := tui.StreamStatus{
		Elapsed: 1 * time.Second,
		Tokens:  0,
	}
	result := tui.FormatStreamStatus(s)
	assert.Contains(t, result, "1s")
	assert.Contains(t, result, "0 tokens")
}

func TestRenderStreamingIndicator(t *testing.T) {
	s := tui.StreamStatus{
		Elapsed: 5 * time.Second,
		Tokens:  120,
	}
	result := tui.RenderStreamingIndicator(0, "coder", s, tui.SpinnerBraille)
	assert.Contains(t, result, "⠋") // first braille frame
	assert.Contains(t, result, "Composing")
	assert.Contains(t, result, "5s")
	assert.Contains(t, result, "120 tokens")
}

func TestRenderCompletionIndicator(t *testing.T) {
	result := tui.RenderCompletionIndicator(8*time.Minute + 35*time.Second)
	assert.Contains(t, result, "✻")
	assert.Contains(t, result, "Crunched for 8m35s")
}

func TestRenderCompletionIndicator_Short(t *testing.T) {
	result := tui.RenderCompletionIndicator(3 * time.Second)
	assert.Contains(t, result, "✻")
	assert.Contains(t, result, "Crunched for 3s")
}

func TestEstimateTokens(t *testing.T) {
	// ~4 chars per token is a rough estimate
	assert.Equal(t, 0, tui.EstimateTokens(0))
	assert.Equal(t, 1, tui.EstimateTokens(4))
	assert.Equal(t, 25, tui.EstimateTokens(100))
	assert.Equal(t, 250, tui.EstimateTokens(1000))
}
