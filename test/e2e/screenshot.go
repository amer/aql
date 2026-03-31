package e2e

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Screenshot — captured terminal state as plain text,
//     NewScreenshot() — constructor from raw text,
//     Contains() — substring check on screenshot text,
//     Line() — access a specific row by index,
//     Save() — write screenshot to a file.
//
// MUST NOT GO HERE:
//   - Terminal interaction or PTY management (terminal.go)
//   - Taking screenshots from a terminal (terminal.go — Screenshot())
//   - HTTP recording (recorder.go)
//
// Q: Should I add a new assertion helper on Screenshot?
// A: Yes, add it here as a method on Screenshot. Keep it pure —
//    no side effects, just query the text.
//
// Q: Where does the actual screenshot capture happen?
// A: Terminal.Screenshot() in terminal.go reads the vt10x state and
//    creates a Screenshot via NewScreenshot().
// ──────────────────────────────────────────────────────────────────

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Screenshot represents a captured terminal state as plain text.
type Screenshot struct {
	Text      string
	Lines     []string
	Timestamp time.Time
}

// NewScreenshot creates a Screenshot from raw terminal text.
func NewScreenshot(text string, ts time.Time) Screenshot {
	return Screenshot{
		Text:      text,
		Lines:     strings.Split(text, "\n"),
		Timestamp: ts,
	}
}

// Contains reports whether the screenshot text contains substr.
func (s Screenshot) Contains(substr string) bool {
	return strings.Contains(s.Text, substr)
}

// Line returns the text at the given row (0-indexed).
// Returns empty string for out-of-bounds rows.
func (s Screenshot) Line(row int) string {
	if row < 0 || row >= len(s.Lines) {
		return ""
	}
	return s.Lines[row]
}

// Save writes the screenshot text to a file, creating parent directories.
func (s Screenshot) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(s.Text+"\n"), 0o644)
}
