package e2e_test

import (
	"testing"
	"time"

	"github.com/amer/aql/test/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTerminal_SpawnAndCapture(t *testing.T) {
	term := e2e.NewTerminal(t,
		e2e.WithBinary("/bin/sh"),
		e2e.WithSize(80, 24),
	)

	term.Send("echo HELLO_E2E\n")
	err := term.WaitFor("HELLO_E2E", 3*time.Second)
	require.NoError(t, err)

	shot := term.Screenshot()
	require.True(t, shot.Contains("HELLO_E2E"))
}

func TestTerminal_Type(t *testing.T) {
	term := e2e.NewTerminal(t,
		e2e.WithBinary("/bin/sh"),
		e2e.WithSize(80, 24),
	)

	// Type a command character by character, then send enter
	term.Type("echo TYPED")
	term.SendKey(e2e.KeyEnter)

	err := term.WaitFor("TYPED", 3*time.Second)
	require.NoError(t, err)
}

func TestTerminal_SendKey(t *testing.T) {
	term := e2e.NewTerminal(t,
		e2e.WithBinary("/bin/sh"),
		e2e.WithSize(80, 24),
	)

	// Send a command followed by Enter key
	term.Send("echo KEYTEST")
	term.SendKey(e2e.KeyEnter)

	err := term.WaitFor("KEYTEST", 3*time.Second)
	require.NoError(t, err)
}

func TestTerminal_WaitFor_Timeout(t *testing.T) {
	term := e2e.NewTerminal(t,
		e2e.WithBinary("/bin/sh"),
		e2e.WithSize(80, 24),
	)

	err := term.WaitFor("THIS_WILL_NEVER_APPEAR", 200*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
	// Error should include last screen state for debugging
	assert.Contains(t, err.Error(), "last screen")
}

func TestTerminal_WaitForFunc(t *testing.T) {
	term := e2e.NewTerminal(t,
		e2e.WithBinary("/bin/sh"),
		e2e.WithSize(80, 24),
	)

	term.Send("echo LINE_ONE\n")

	err := term.WaitForFunc(func(s e2e.Screenshot) bool {
		return s.Contains("LINE_ONE")
	}, 3*time.Second)
	require.NoError(t, err)
}

func TestTerminal_Screenshot_TrimsTrailingBlanks(t *testing.T) {
	term := e2e.NewTerminal(t,
		e2e.WithBinary("/bin/sh"),
		e2e.WithSize(80, 24),
	)

	term.Send("echo SHORT\n")
	require.NoError(t, term.WaitFor("SHORT", 3*time.Second))

	shot := term.Screenshot()
	// Should not have 24 lines of whitespace — trailing blanks are trimmed
	assert.Less(t, len(shot.Lines), 24)
}

func TestTerminal_SaveScreenshot(t *testing.T) {
	term := e2e.NewTerminal(t,
		e2e.WithBinary("/bin/sh"),
		e2e.WithSize(80, 24),
	)

	term.Send("echo SAVED\n")
	require.NoError(t, term.WaitFor("SAVED", 3*time.Second))

	shot := term.SaveScreenshot("test-save")
	assert.True(t, shot.Contains("SAVED"))
	// Counter increments
	shot2 := term.SaveScreenshot("test-save-2")
	assert.True(t, shot2.Contains("SAVED"))
}

func TestTerminal_WithSize(t *testing.T) {
	term := e2e.NewTerminal(t,
		e2e.WithBinary("/bin/sh"),
		e2e.WithSize(40, 10),
	)

	// Shell should work in a small terminal
	term.Send("echo SMALL\n")
	require.NoError(t, term.WaitFor("SMALL", 3*time.Second))
}

func TestTerminal_WithEnv(t *testing.T) {
	term := e2e.NewTerminal(t,
		e2e.WithBinary("/bin/sh"),
		e2e.WithSize(80, 24),
		e2e.WithEnv("MY_TEST_VAR=hello123"),
	)

	term.Send("echo $MY_TEST_VAR\n")
	require.NoError(t, term.WaitFor("hello123", 3*time.Second))
}

func TestTerminal_MultipleCommands(t *testing.T) {
	term := e2e.NewTerminal(t,
		e2e.WithBinary("/bin/sh"),
		e2e.WithSize(80, 24),
	)

	term.Send("echo FIRST\n")
	require.NoError(t, term.WaitFor("FIRST", 3*time.Second))

	term.Send("echo SECOND\n")
	require.NoError(t, term.WaitFor("SECOND", 3*time.Second))

	shot := term.Screenshot()
	assert.True(t, shot.Contains("FIRST"))
	assert.True(t, shot.Contains("SECOND"))
}
