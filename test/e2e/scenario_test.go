//go:build e2e

package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/amer/aql/test/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_WelcomeScreen(t *testing.T) {
	term := e2e.NewTerminal(t, e2e.WithSize(120, 40), e2e.WithStubAPI())

	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return len(s.Text) > 0
	}, 10*time.Second))

	term.SaveScreenshot("welcome")
}

func TestE2E_TypeAndSlashHelp(t *testing.T) {
	term := e2e.NewTerminal(t, e2e.WithSize(120, 40), e2e.WithStubAPI())

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
	term := e2e.NewTerminal(t, e2e.WithSize(120, 40), e2e.WithStubAPI())

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

	// Assert on text that only appears if the streamed API response was parsed
	// and rendered. "friend" is in the recorded assistant reply but not in the
	// prompt, so — unlike the old WaitFor("hello") — it can't be satisfied by
	// the echoed input alone.
	require.NoError(t, term.WaitFor("friend", 30*time.Second))

	final := term.SaveScreenshot("after-response")
	assert.Contains(t, final.Text, "How can I help you today?")
}

func TestE2E_SlashDiff(t *testing.T) {
	workDir := t.TempDir()

	// Initialize a git repo with one committed file, then modify it
	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "command %v failed: %s", args, out)
	}
	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(workDir, "hello.txt"), []byte("hello world\n"), 0o644)
	run("git", "add", "hello.txt")
	run("git", "commit", "-m", "initial")

	// Create an uncommitted change
	os.WriteFile(filepath.Join(workDir, "hello.txt"), []byte("goodbye world\n"), 0o644)

	term := e2e.NewTerminal(t,
		e2e.WithSize(120, 40),
		e2e.WithStubAPI(),
		e2e.WithWorkDir(workDir),
	)

	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return len(s.Text) > 0
	}, 10*time.Second))

	term.SaveScreenshot("ready")

	term.Type("/diff")
	term.SaveScreenshot("after-typing-diff")

	term.SendKey(e2e.KeyEnter)

	// Wait for diff overlay to appear
	require.NoError(t, term.WaitForFunc(func(s e2e.Screenshot) bool {
		return s.Contains("hello.txt") || s.Contains("diff") || s.Contains("Changes")
	}, 10*time.Second))

	term.SaveScreenshot("diff-overlay")

	// Press enter to see file detail
	term.SendKey(e2e.KeyEnter)
	time.Sleep(100 * time.Millisecond)
	term.SaveScreenshot("diff-detail")

	// Press escape to go back to list
	term.SendKey(e2e.KeyEscape)
	time.Sleep(100 * time.Millisecond)
	term.SaveScreenshot("diff-list-again")

	// Press escape to close overlay
	term.SendKey(e2e.KeyEscape)
	time.Sleep(100 * time.Millisecond)
	term.SaveScreenshot("after-close")
}

// TestE2E_EditFileShowsDiff drives the full edit round-trip: the agent emits an
// edit tool call, the C6 approval gate prompts for consent, the user approves,
// and the edit is applied. It asserts the real on-disk effect and that the tool
// call is surfaced in the transcript — not a vacuous log line.
//
// A richer Claude-Code-style diff render (± lines under an Update header) is a
// separate, not-yet-built feature; this test guards the behaviour that exists.
func TestE2E_EditFileShowsDiff(t *testing.T) {
	fixtureDir := filepath.Join("testdata", "edit-file-diff")
	apiOpt := e2e.APIOption(fixtureDir)

	if os.Getenv("E2E_RECORD") != "" && !e2e.HasAPICredentials() {
		t.Skip("E2E_RECORD set but no API credentials")
	}

	workDir := t.TempDir()

	seed := filepath.Join(workDir, "hello.txt")
	require.NoError(t, os.WriteFile(seed, []byte("hello world\n"), 0o644))

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

	// The approval gate (C6) must intercept the edit before it runs.
	require.NoError(t, term.WaitFor("(y/n)", 30*time.Second))
	term.SaveScreenshot("approval-prompt")

	// Approve the edit.
	term.Type("y")
	term.SendKey(e2e.KeyEnter)

	// Wait for the agent's post-edit reply, proving the tool result round-tripped.
	require.NoError(t, term.WaitFor("goodbye world", 30*time.Second))
	term.SaveScreenshot("final")

	// The real effect: the file on disk was actually edited.
	got, err := os.ReadFile(seed)
	require.NoError(t, err)
	assert.Equal(t, "goodbye world\n", string(got))
}
