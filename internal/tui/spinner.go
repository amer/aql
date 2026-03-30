package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// SpinnerTickMsg triggers a spinner frame advance.
type SpinnerTickMsg struct{}

// SpinnerTick returns a command that ticks the spinner.
func SpinnerTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return SpinnerTickMsg{}
	})
}

// RenderSpinner renders the spinner at the given frame index with a label.
func RenderSpinner(frame int, label string) string {
	f := spinnerFrames[frame%len(spinnerFrames)]
	return SpinnerStyle.Render(f) + " " + MutedStyle.Render(label)
}
