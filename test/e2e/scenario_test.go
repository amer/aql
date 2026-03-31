//go:build e2e

package e2e_test

import (
	"os"
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
