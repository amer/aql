package tui

import (
	"math/rand/v2"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// SpinnerType identifies a spinner animation style.
type SpinnerType int

const (
	SpinnerBraille      SpinnerType = iota // ⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏
	SpinnerCircle                          // ◡⊙◠
	SpinnerArc                             // ◜◠◝◞◡◟
	SpinnerToggle8                         // ◍◌
	SpinnerToggle7                         // ⦾⦿
	SpinnerCircleHalves                    // ◐◓◑◒
)

// Spinner holds the definition for a spinner animation.
type Spinner struct {
	Name     string
	Frames   []string
	Interval time.Duration
}

var spinners = map[SpinnerType]Spinner{
	SpinnerBraille: {
		Name:     "braille",
		Frames:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		Interval: 80 * time.Millisecond,
	},
	SpinnerCircle: {
		Name:     "circle",
		Frames:   []string{"◡", "⊙", "◠"},
		Interval: 120 * time.Millisecond,
	},
	SpinnerArc: {
		Name:     "arc",
		Frames:   []string{"◜", "◠", "◝", "◞", "◡", "◟"},
		Interval: 100 * time.Millisecond,
	},
	SpinnerToggle8: {
		Name:     "toggle8",
		Frames:   []string{"◍", "◌"},
		Interval: 100 * time.Millisecond,
	},
	SpinnerToggle7: {
		Name:     "toggle7",
		Frames:   []string{"⦾", "⦿"},
		Interval: 80 * time.Millisecond,
	},
	SpinnerCircleHalves: {
		Name:     "circleHalves",
		Frames:   []string{"◐", "◓", "◑", "◒"},
		Interval: 50 * time.Millisecond,
	},
}

// SpinnerDef returns the definition for the given spinner type.
func SpinnerDef(st SpinnerType) Spinner {
	if s, ok := spinners[st]; ok {
		return s
	}
	return spinners[SpinnerBraille]
}

// SpinnerTypes returns all available spinner types in order.
func SpinnerTypes() []SpinnerType {
	return []SpinnerType{
		SpinnerBraille,
		SpinnerCircle,
		SpinnerArc,
		SpinnerToggle8,
		SpinnerToggle7,
		SpinnerCircleHalves,
	}
}

// RandomSpinnerType returns a random spinner type.
func RandomSpinnerType() SpinnerType {
	types := SpinnerTypes()
	return types[rand.IntN(len(types))]
}

// SpinnerFrameAt returns the frame string for the given type and frame index.
func SpinnerFrameAt(st SpinnerType, frame int) string {
	def := SpinnerDef(st)
	return def.Frames[frame%len(def.Frames)]
}

// SpinnerTickMsg triggers a spinner frame advance.
type SpinnerTickMsg struct{}

// SpinnerTick returns a command that ticks the spinner using the default (braille) interval.
func SpinnerTick() tea.Cmd {
	return SpinnerTickFor(SpinnerBraille)
}

// SpinnerTickFor returns a command that ticks the spinner at the given type's interval.
func SpinnerTickFor(st SpinnerType) tea.Cmd {
	def := SpinnerDef(st)
	return tea.Tick(def.Interval, func(t time.Time) tea.Msg {
		return SpinnerTickMsg{}
	})
}

// RenderSpinner renders the default braille spinner at the given frame index with a label.
func RenderSpinner(frame int, label string) string {
	return RenderSpinnerWithType(frame, label, SpinnerBraille)
}

// RenderSpinnerWithType renders the spinner of the given type at the given frame index with a label.
func RenderSpinnerWithType(frame int, label string, st SpinnerType) string {
	f := SpinnerFrameAt(st, frame)
	return SpinnerStyle.Render(f) + " " + MutedStyle.Render(label)
}
