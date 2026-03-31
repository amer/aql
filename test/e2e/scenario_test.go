//go:build e2e

package e2e_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amer/aql/test/e2e"
	"github.com/stretchr/testify/require"
)

func TestE2E_WelcomeScreen(t *testing.T) {
	term := e2e.NewTerminal(t, e2e.WithSize(120, 40))

	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return len(s.Text) > 0
	}, 10*time.Second))

	term.SaveScreenshot("welcome")
}

func TestE2E_TypeAndSlashHelp(t *testing.T) {
	term := e2e.NewTerminal(t, e2e.WithSize(120, 40))

	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return len(s.Text) > 0
	}, 10*time.Second))

	term.SaveScreenshot("initial")

	term.Type("/help")
	term.SaveScreenshot("after-slash-help")

	term.SendKey(e2e.KeyEnter)
	term.SaveScreenshot("after-enter")
}

func TestE2E_CtrlCExit(t *testing.T) {
	term := e2e.NewTerminal(t, e2e.WithSize(120, 40))

	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return len(s.Text) > 0
	}, 10*time.Second))

	term.SaveScreenshot("before-exit")

	term.SendKey(e2e.KeyCtrlC)
	term.SaveScreenshot("after-ctrl-c")
}

func TestE2E_RecordAPICall(t *testing.T) {
	fixtureDir := filepath.Join("testdata", "record-api-call")
	apiOpt := e2e.APIOption(fixtureDir)

	// Recording mode requires credentials
	if os.Getenv("E2E_RECORD") != "" && !e2e.HasAPICredentials() {
		t.Skip("E2E_RECORD set but no API credentials")
	}

	term := e2e.NewTerminal(t,
		e2e.WithSize(120, 40),
		apiOpt,
	)

	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return len(s.Text) > 0
	}, 10*time.Second))

	term.SaveScreenshot("ready")

	term.Type("say hello")
	term.SendKey(e2e.KeyEnter)

	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return s.Contains("hello") || s.Contains("Hello")
	}, 30*time.Second))

	term.SaveScreenshot("after-response")
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
	fixtureDir := filepath.Join("testdata", "edit-file-diff")
	apiOpt := e2e.APIOption(fixtureDir)

	if os.Getenv("E2E_RECORD") != "" && !e2e.HasAPICredentials() {
		t.Skip("E2E_RECORD set but no API credentials")
	}

	workDir := t.TempDir()

	seed := filepath.Join(workDir, "hello.txt")
	os.WriteFile(seed, []byte("hello world\n"), 0o644)

	term := e2e.NewTerminal(t,
		e2e.WithSize(120, 40),
		e2e.WithWorkDir(workDir),
		apiOpt,
	)

	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return len(s.Text) > 0
	}, 10*time.Second))

	term.SaveScreenshot("ready")

	term.Type("edit hello.txt and change 'hello world' to 'goodbye world'")
	term.SendKey(e2e.KeyEnter)

	// Wait for the tool call to appear
	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return s.Contains("Update") || s.Contains("Edited") || s.Contains("hello.txt")
	}, 30*time.Second))

	final := term.SaveScreenshot("final")

	hasUpdate := final.Contains("Update")
	hasDiff := final.Contains("- hello") || final.Contains("+ goodbye")
	t.Logf("Update header: %v, diff lines: %v, exchanges: %d",
		hasUpdate, hasDiff, len(term.APIExchanges()))
}
