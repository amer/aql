//go:build e2e

package e2e_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amer/aql/test/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_WelcomeScreen(t *testing.T) {
	term := e2e.NewTerminal(t, e2e.WithSize(120, 40))

	// The app should render its TUI — wait for any content
	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return len(s.Text) > 0
	}, 10*time.Second))

	shot := term.SaveScreenshot("welcome")
	t.Logf("Welcome screen:\n%s", shot.Text)
}

func TestE2E_TypeAndSlashHelp(t *testing.T) {
	term := e2e.NewTerminal(t, e2e.WithSize(120, 40))

	// Wait for TUI to render
	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return len(s.Text) > 0
	}, 10*time.Second))

	term.SaveScreenshot("initial")

	// Type /help to trigger command palette
	term.Type("/help")
	time.Sleep(200 * time.Millisecond)
	term.SaveScreenshot("after-slash-help")

	// Submit with enter
	term.SendKey(e2e.KeyEnter)
	time.Sleep(500 * time.Millisecond)
	term.SaveScreenshot("after-enter")
}

func TestE2E_CtrlCExit(t *testing.T) {
	term := e2e.NewTerminal(t, e2e.WithSize(120, 40))

	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return len(s.Text) > 0
	}, 10*time.Second))

	term.SaveScreenshot("before-exit")

	// Ctrl+C should trigger exit
	term.SendKey(e2e.KeyCtrlC)
	time.Sleep(500 * time.Millisecond)
	term.SaveScreenshot("after-ctrl-c")

	// Logs may be empty if the app exited before writing (e.g., no API key)
	logs := term.Logs()
	t.Logf("Log length: %d bytes", len(logs))
}

func TestE2E_RecordAPICall(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set — skipping API recording test")
	}

	term := e2e.NewTerminal(t,
		e2e.WithSize(120, 40),
		e2e.WithRecordAPI(),
	)

	// Wait for TUI to render (with API key, app should start normally)
	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return len(s.Text) > 0
	}, 10*time.Second))

	term.SaveScreenshot("ready")

	// Send a simple prompt
	term.Type("say hello")
	term.SendKey(e2e.KeyEnter)

	// Wait for the response to start streaming
	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return s.Contains("hello") || s.Contains("Hello")
	}, 30*time.Second))

	term.SaveScreenshot("after-response")

	// Verify API calls were recorded
	exchanges := term.APIExchanges()
	assert.NotEmpty(t, exchanges, "should have recorded API exchanges")
	t.Logf("Recorded %d API exchanges", len(exchanges))
	for i, ex := range exchanges {
		t.Logf("  [%d] %s %s → %d (%s)", i+1, ex.Method, ex.Path, ex.StatusCode, ex.Duration)
	}
}

// TestE2E_EditFileShowsDiff verifies that when the agent edits a file,
// the TUI renders the tool call with a diff showing added/removed lines —
// not just "Edited file.go".
//
// Expected (Claude Code style):
//
//	⏺ Update(hello.txt)
//	  ⎿  Updated 2 lines
//	     - old line
//	     + new line
//
// Current behavior: only shows "Edited hello.txt" with no diff.
func TestE2E_EditFileShowsDiff(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set — skipping edit diff test")
	}

	workDir := t.TempDir()

	// Create a seed file for the agent to edit
	seed := filepath.Join(workDir, "hello.txt")
	os.WriteFile(seed, []byte("hello world\n"), 0o644)

	term := e2e.NewTerminal(t,
		e2e.WithSize(120, 40),
		e2e.WithWorkDir(workDir),
		e2e.WithRecordAPI(),
	)

	// Wait for TUI to be ready
	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return len(s.Text) > 0
	}, 10*time.Second))

	term.SaveScreenshot("ready")

	// Ask the agent to edit the file
	term.Type("edit hello.txt and change 'hello world' to 'goodbye world'")
	term.SendKey(e2e.KeyEnter)

	// Wait for the tool call to appear — look for "Update" (the display name for edit)
	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return s.Contains("Update") || s.Contains("Edited") || s.Contains("hello.txt")
	}, 30*time.Second))

	term.SaveScreenshot("after-tool-call")

	// Give it a moment to finish rendering
	time.Sleep(2 * time.Second)
	term.SaveScreenshot("settled")

	// Take a final screenshot and log the full screen for analysis
	final := term.Screenshot()
	t.Logf("Final screen:\n%s", final.Text)

	// Check what's visible — these are diagnostic, not strict assertions.
	// The goal is to capture the current behavior and identify what's missing.
	if final.Contains("Update") {
		t.Log("✓ 'Update' header found")
	} else {
		t.Log("✗ 'Update' header NOT found")
	}
	if final.Contains("hello.txt") {
		t.Log("✓ 'hello.txt' filename found")
	} else {
		t.Log("✗ 'hello.txt' filename NOT found")
	}

	// These are what we WANT to see but currently don't
	if final.Contains("goodbye") || final.Contains("+ goodbye") {
		t.Log("✓ diff shows new content")
	} else {
		t.Log("✗ diff does NOT show new content — this is the bug")
	}
	if final.Contains("- hello") || final.Contains("+ goodbye") {
		t.Log("✓ diff shows added/removed lines")
	} else {
		t.Log("✗ diff does NOT show added/removed lines — needs FormatToolSummary enhancement")
	}

	// Log API exchanges for debugging
	exchanges := term.APIExchanges()
	t.Logf("Recorded %d API exchanges", len(exchanges))
}
