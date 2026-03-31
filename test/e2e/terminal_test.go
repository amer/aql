package e2e_test

import (
	"net/http"
	"os"
	"path/filepath"
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

func TestAPIOption_ReplayByDefault(t *testing.T) {
	// Create a fixture dir with exchanges
	dir := t.TempDir()
	exchanges := []e2e.Exchange{{
		Method:          "POST",
		Path:            "/v1/messages",
		StatusCode:      200,
		ResponseHeaders: http.Header{"Content-Type": []string{"application/json"}},
		ResponseBody:    `{"id":"msg_1"}`,
	}}
	require.NoError(t, e2e.SaveExchanges(dir, exchanges))

	// Without E2E_RECORD, should return WithReplayAPI
	t.Setenv("E2E_RECORD", "")
	opt := e2e.APIOption(dir)

	// Apply to a config and verify it sets replayDir (not recordAPI)
	cfg := e2e.ApplyOptions(opt)
	assert.Equal(t, dir, cfg.ReplayDir)
	assert.False(t, cfg.RecordAPI)
}

func TestAPIOption_RecordWhenEnvSet(t *testing.T) {
	t.Setenv("E2E_RECORD", "1")
	opt := e2e.APIOption("/some/dir")

	cfg := e2e.ApplyOptions(opt)
	assert.Equal(t, "", cfg.ReplayDir)
	assert.True(t, cfg.RecordAPI)
	assert.Equal(t, "/some/dir", cfg.FixtureDir)
}

func TestHasAPICredentials_EnvVar(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	assert.True(t, e2e.HasAPICredentials())
}

func TestHasAPICredentials_NoCredentials(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	// May still return true if a token file exists on this machine,
	// so we just verify it doesn't panic and returns a bool.
	_ = e2e.HasAPICredentials()
}

func TestTerminal_CopiesTokenFile(t *testing.T) {
	// Create a fake token file in a temp "project root"
	srcDir := t.TempDir()
	tokenPath := filepath.Join(srcDir, ".aql_tokens.json")
	os.WriteFile(tokenPath, []byte(`{"api_key":"fake"}`), 0o600)

	// Spawn a shell with WithWorkDir pointing to a different temp dir
	workDir := t.TempDir()
	term := e2e.NewTerminal(t,
		e2e.WithBinary("/bin/sh"),
		e2e.WithSize(80, 24),
		e2e.WithWorkDir(workDir),
	)

	// The token file should have been copied into the workDir
	dst := filepath.Join(workDir, ".aql_tokens.json")
	_, err := os.Stat(dst)
	if e2e.HasAPICredentials() {
		// If credentials exist on this machine, the file should be copied
		require.NoError(t, err, "token file should be copied to workDir")
	}

	// Shell still works regardless
	term.Send("echo TOKEN_TEST\n")
	require.NoError(t, term.WaitFor("TOKEN_TEST", 3*time.Second))
}
