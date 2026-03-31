package tui

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - OSC52Copy — terminal clipboard via escape sequence,
//     ExecCopy — platform-native clipboard (pbcopy/xclip/xsel),
//     copyToClipboard tea.Cmd, ClipboardMsg.
//
// MUST NOT GO HERE:
//   - Selection logic (selection.go), rendering, state mutation.
// ──────────────────────────────────────────────────────────────────

import (
	"encoding/base64"
	"fmt"
	"io"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// OSC52Copy writes an OSC 52 escape sequence to copy text to the system clipboard.
// This works in most modern terminals (iTerm2, kitty, WezTerm, Alacritty, etc.)
// without needing external tools like pbcopy or xclip.
func OSC52Copy(w io.Writer, text string) {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	fmt.Fprintf(w, "\x1b]52;c;%s\x07", encoded)
}

// ExecCopy copies text to the system clipboard using platform-native commands.
// On macOS it uses pbcopy; on Linux it tries xclip then xsel.
func ExecCopy(text string) error {
	var cmd *exec.Cmd

	if path, err := exec.LookPath("pbcopy"); err == nil {
		cmd = exec.Command(path)
	} else if path, err := exec.LookPath("xclip"); err == nil {
		cmd = exec.Command(path, "-selection", "clipboard")
	} else if path, err := exec.LookPath("xsel"); err == nil {
		cmd = exec.Command(path, "--clipboard", "--input")
	} else {
		return fmt.Errorf("no clipboard command found (tried pbcopy, xclip, xsel)")
	}

	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// ClipboardMsg is sent after text is copied to the clipboard.
type ClipboardMsg struct {
	Text string
}

// copyToClipboard returns a tea.Cmd that copies text to the system clipboard.
// Uses native clipboard commands (pbcopy/xclip/xsel) which work reliably
// in all terminal modes including alt screen.
func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		_ = ExecCopy(text)
		return ClipboardMsg{Text: text}
	}
}
