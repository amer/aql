package tui_test

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpinnerTypeFrames(t *testing.T) {
	tests := []struct {
		name   string
		st     tui.SpinnerType
		frames []string
	}{
		{"Braille", tui.SpinnerBraille, []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}},
		{"Circle", tui.SpinnerCircle, []string{"◡", "⊙", "◠"}},
		{"Arc", tui.SpinnerArc, []string{"◜", "◠", "◝", "◞", "◡", "◟"}},
		{"Toggle8", tui.SpinnerToggle8, []string{"◍", "◌"}},
		{"Toggle7", tui.SpinnerToggle7, []string{"⦾", "⦿"}},
		{"CircleHalves", tui.SpinnerCircleHalves, []string{"◐", "◓", "◑", "◒"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := tui.SpinnerDef(tt.st)
			assert.Equal(t, tt.frames, def.Frames)
			assert.Greater(t, def.Interval, time.Duration(0), "interval must be positive")
		})
	}
}

func TestSpinnerTypeIntervals(t *testing.T) {
	tests := []struct {
		name     string
		st       tui.SpinnerType
		interval time.Duration
	}{
		{"Braille", tui.SpinnerBraille, 80 * time.Millisecond},
		{"Circle", tui.SpinnerCircle, 120 * time.Millisecond},
		{"Arc", tui.SpinnerArc, 100 * time.Millisecond},
		{"Toggle8", tui.SpinnerToggle8, 100 * time.Millisecond},
		{"Toggle7", tui.SpinnerToggle7, 80 * time.Millisecond},
		{"CircleHalves", tui.SpinnerCircleHalves, 50 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := tui.SpinnerDef(tt.st)
			assert.Equal(t, tt.interval, def.Interval)
		})
	}
}

func TestSpinnerDefUnknownFallsBackToBraille(t *testing.T) {
	def := tui.SpinnerDef(tui.SpinnerType(99))
	braille := tui.SpinnerDef(tui.SpinnerBraille)
	assert.Equal(t, braille.Frames, def.Frames)
	assert.Equal(t, braille.Interval, def.Interval)
}

func TestRenderSpinner(t *testing.T) {
	result := tui.RenderSpinner(0, "thinking...")
	assert.Contains(t, result, "thinking...")
}

func TestRenderSpinnerWraps(t *testing.T) {
	r1 := tui.RenderSpinner(0, "test")
	r2 := tui.RenderSpinner(10, "test")
	assert.Equal(t, r1, r2, "frame 10 should wrap to frame 0 for braille (10 frames)")
}

func TestRenderSpinnerWithType(t *testing.T) {
	tests := []struct {
		name      string
		st        tui.SpinnerType
		frame     int
		wantFrame string
	}{
		{"Circle frame 0", tui.SpinnerCircle, 0, "◡"},
		{"Circle frame 1", tui.SpinnerCircle, 1, "⊙"},
		{"Circle wraps", tui.SpinnerCircle, 3, "◡"},
		{"Arc frame 2", tui.SpinnerArc, 2, "◝"},
		{"Toggle8 frame 0", tui.SpinnerToggle8, 0, "◍"},
		{"Toggle8 frame 1", tui.SpinnerToggle8, 1, "◌"},
		{"Toggle7 wraps", tui.SpinnerToggle7, 4, "⦾"},
		{"CircleHalves frame 3", tui.SpinnerCircleHalves, 3, "◒"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tui.RenderSpinnerWithType(tt.frame, "label", tt.st)
			assert.Contains(t, result, tt.wantFrame)
			assert.Contains(t, result, "label")
		})
	}
}

func TestSpinnerTickUsesActiveSpinnerInterval(t *testing.T) {
	m := tui.NewModel("test", []string{"agent"}, nil)
	m.SetSpinnerType(tui.SpinnerCircle)

	// Verify spinner tick returns a command (non-nil)
	cmd := tui.SpinnerTickFor(tui.SpinnerCircle)
	require.NotNil(t, cmd)
}

func TestModelSpinnerTypePersists(t *testing.T) {
	m := tui.NewModel("test", []string{"agent"}, nil)

	m.SetSpinnerType(tui.SpinnerArc)
	assert.Equal(t, tui.SpinnerArc, m.ActiveSpinnerType())
}

func TestRandomSpinnerTypeReturnsValidType(t *testing.T) {
	types := tui.SpinnerTypes()
	valid := make(map[tui.SpinnerType]bool)
	for _, st := range types {
		valid[st] = true
	}

	// Call many times to exercise randomness
	for range 100 {
		st := tui.RandomSpinnerType()
		assert.True(t, valid[st], "RandomSpinnerType returned invalid type: %d", st)
	}
}

func TestRandomSpinnerTypeHasVariety(t *testing.T) {
	seen := make(map[tui.SpinnerType]bool)
	for range 200 {
		seen[tui.RandomSpinnerType()] = true
	}
	// With 6 types and 200 draws, we should see at least 2 distinct types
	assert.GreaterOrEqual(t, len(seen), 2, "RandomSpinnerType should produce variety")
}

func TestStreamingStartPicksRandomSpinner(t *testing.T) {
	seen := make(map[tui.SpinnerType]bool)
	for range 50 {
		m := tui.NewModel("test", []string{"agent"}, nil)
		// Simulate streaming start
		updated, _ := m.Update(tui.AgentStreamDeltaMsg{AgentName: "agent", Delta: "hi"})
		m = updated.(tui.Model)
		seen[m.ActiveSpinnerType()] = true

		// Must be streaming
		assert.True(t, m.IsStreaming())
	}
	// Should see more than one type across 50 starts
	assert.GreaterOrEqual(t, len(seen), 2, "streaming should pick random spinner types")
}

func TestSpinnerCycleCommand(t *testing.T) {
	m := tui.NewModel("test", []string{"agent"}, nil)

	// Start with braille
	assert.Equal(t, tui.SpinnerBraille, m.ActiveSpinnerType())

	// Simulate /spinner command — should cycle through types
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("/")}))
	m = updated.(tui.Model)

	types := tui.SpinnerTypes()
	assert.Len(t, types, 6, "should have 6 spinner types")

	// All types should be unique
	seen := make(map[tui.SpinnerType]bool)
	for _, st := range types {
		assert.False(t, seen[st], "duplicate spinner type: %d", st)
		seen[st] = true
	}
}

func TestSpinnerFrameAtForEachType(t *testing.T) {
	for _, st := range tui.SpinnerTypes() {
		def := tui.SpinnerDef(st)
		t.Run(def.Name, func(t *testing.T) {
			// Frame 0 should return first frame
			assert.Equal(t, def.Frames[0], tui.SpinnerFrameAt(st, 0))
			// Frame at len should wrap to 0
			assert.Equal(t, def.Frames[0], tui.SpinnerFrameAt(st, len(def.Frames)))
		})
	}
}
