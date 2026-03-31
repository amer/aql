// Package e2e provides a test harness for end-to-end testing of the aql binary.
// It spawns the binary in a PTY, interacts with it via keystrokes, and captures
// text screenshots of the terminal state for offline analysis.
package e2e

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Key type — represents terminal keys as ANSI escape sequences,
//     Key constants — KeyEnter, KeyEscape, KeyTab, KeyCtrlC, arrow keys, etc.
//
// MUST NOT GO HERE:
//   - Terminal spawning or PTY management (terminal.go)
//   - Screenshot capture (screenshot.go)
//   - HTTP recording (recorder.go)
//
// Q: Should I add a new special key?
// A: Yes, add it here as a Key constant with its ANSI escape sequence.
//
// Q: Can I add key combination helpers here?
// A: Only if they're raw ANSI sequences. Higher-level input helpers
//    belong on Terminal methods.
// ──────────────────────────────────────────────────────────────────

// Key represents a terminal key as its ANSI escape sequence.
type Key string

const (
	KeyEnter     Key = "\r"
	KeyEscape    Key = "\x1b"
	KeyTab       Key = "\t"
	KeyBackspace Key = "\x7f"
	KeyCtrlC     Key = "\x03"
	KeyCtrlD     Key = "\x04"
	KeyCtrlL     Key = "\x0c"
	KeyUp        Key = "\x1b[A"
	KeyDown      Key = "\x1b[B"
	KeyRight     Key = "\x1b[C"
	KeyLeft      Key = "\x1b[D"
	KeyPgUp      Key = "\x1b[5~"
	KeyPgDown    Key = "\x1b[6~"
	KeyHome      Key = "\x1b[H"
	KeyEnd       Key = "\x1b[F"
)
