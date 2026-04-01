package e2e

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Terminal — wraps PTY-spawned process with vt10x emulator,
//     NewTerminal() — spawns the aql binary in a PTY,
//     Send() / SendKey() / Type() — write input to the PTY,
//     Screenshot() / SaveScreenshot() — capture terminal state,
//     WaitFor() / WaitForFunc() — poll until condition met,
//     APIExchanges() / Logs() — access test artifacts,
//     setupAPIProxy() — configures stub/replay/record proxy,
//     copyTokenFile() — copies credentials into workDir.
//
// MUST NOT GO HERE:
//   - Screenshot type and methods (screenshot.go)
//   - Exchange type and serialization (exchange.go)
//   - Recorder proxy (recorder.go), Replayer server (replayer.go)
//   - Terminal configuration options (option.go)
//   - Key constants (key.go)
//   - Binary build caching (build.go)
//   - Session/artifact directory management (session.go)
//   - Credential detection (credential.go)
//
// Q: How do I add a new interaction method?
// A: Add it as a method on *Terminal here. Use t.ptyFile for raw I/O
//    and t.vt for reading terminal state.
//
// Q: Where do test artifacts go?
// A: test/e2e/artifacts/<TestName>/. Created automatically by
//    NewTerminal(). Use SaveScreenshot() or APIExchanges().
// ──────────────────────────────────────────────────────────────────

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pty "github.com/creack/pty/v2"
	"github.com/hinshun/vt10x"
)

// Terminal wraps a PTY-spawned process and a virtual terminal emulator.
type Terminal struct {
	cmd        *exec.Cmd
	ptyFile    *os.File
	vt         vt10x.Terminal
	done       chan struct{}
	cols       int
	rows       int
	artDir     string
	workDir    string
	counter    int
	recorder   *Recorder // points to shared session recorder, nil if disabled
	fixtureDir string    // where to save recorded fixtures on cleanup
}

// NewTerminal spawns the aql binary in a PTY and returns a Terminal for
// interacting with it. Cleanup is automatic via t.Cleanup().
func NewTerminal(t *testing.T, opts ...Option) *Terminal {
	t.Helper()

	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	binary := cfg.binary
	if binary == "" {
		var err error
		binary, err = ensureBinary()
		if err != nil {
			t.Fatalf("e2e: build binary: %v", err)
		}
	}

	if cfg.workDir == "" {
		cfg.workDir = t.TempDir()
	}

	copyTokenFile(cfg.workDir)
	recorder := setupAPIProxy(t, &cfg)

	cmd := exec.Command(binary)
	cmd.Dir = cfg.workDir
	// CI=1 tells termenv to skip the OSC background-color query, which
	// otherwise blocks for 5 s waiting for a response that vt10x never sends.
	cmd.Env = append(os.Environ(), append([]string{"CI=1"}, cfg.env...)...)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(cfg.cols),
		Rows: uint16(cfg.rows),
	})
	if err != nil {
		t.Fatalf("e2e: start pty: %v", err)
	}

	artDir := filepath.Join(sessionArtifactsDir(), t.Name())
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		t.Fatalf("e2e: create artifacts dir: %v", err)
	}

	term := &Terminal{
		cmd:        cmd,
		ptyFile:    ptmx,
		vt:         vt10x.New(vt10x.WithSize(cfg.cols, cfg.rows)),
		done:       make(chan struct{}),
		cols:       cfg.cols,
		rows:       cfg.rows,
		artDir:     artDir,
		workDir:    cfg.workDir,
		recorder:   recorder,
		fixtureDir: cfg.fixtureDir,
	}

	go term.readLoop()
	t.Cleanup(func() { term.cleanup() })

	return term
}

// copyTokenFile copies the token file into workDir so the binary can find credentials.
func copyTokenFile(workDir string) {
	src := findTokenFile()
	if src == "" {
		return
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(workDir, tokenFile), data, 0o600) //nolint:errcheck
}

// setupAPIProxy configures the API proxy based on the config (stub, replay, or record)
// and appends the ANTHROPIC_BASE_URL to cfg.env. Returns the recorder if recording.
func setupAPIProxy(t *testing.T, cfg *config) *Recorder {
	t.Helper()

	if cfg.stubAPI {
		stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`)) //nolint:errcheck
		}))
		t.Cleanup(stub.Close)
		cfg.env = append(cfg.env, "ANTHROPIC_BASE_URL="+stub.URL)
		return nil
	}

	if cfg.replayDir != "" {
		exchanges, err := LoadExchanges(cfg.replayDir)
		if err != nil {
			t.Skipf("e2e: no fixtures in %s (run with E2E_RECORD=1 to capture): %v", cfg.replayDir, err)
		}
		replayServer := httptest.NewServer(NewReplayer(exchanges))
		t.Cleanup(replayServer.Close)
		cfg.env = append(cfg.env, "ANTHROPIC_BASE_URL="+replayServer.URL)
		return nil
	}

	if cfg.recordAPI {
		recorder, proxy := ensureRecorder()
		cfg.env = append(cfg.env, "ANTHROPIC_BASE_URL="+proxy.URL)
		return recorder
	}

	return nil
}

func (t *Terminal) readLoop() {
	defer close(t.done)
	buf := make([]byte, 4096)
	for {
		n, err := t.ptyFile.Read(buf)
		if n > 0 {
			// vt10x.Write acquires its own internal lock
			t.vt.Write(buf[:n]) //nolint:errcheck
		}
		if err != nil {
			return
		}
	}
}

func (t *Terminal) cleanup() {
	if t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
		_ = t.cmd.Wait()
	}

	t.ptyFile.Close()
	<-t.done

	// Save API recordings
	if t.recorder != nil {
		apiDir := filepath.Join(sessionArtifactsDir(), "api")
		_ = t.recorder.SaveJSON(apiDir)

		// Save to fixture dir for replay in future runs
		if t.fixtureDir != "" {
			_ = t.recorder.SaveJSON(t.fixtureDir)
		}
	}

	// Copy application log to artifacts
	logSrc := filepath.Join(t.workDir, "aql.log")
	if data, err := os.ReadFile(logSrc); err == nil {
		_ = os.WriteFile(filepath.Join(t.artDir, "aql.log"), data, 0o644)
	}
}

// Send writes raw bytes to the PTY.
func (t *Terminal) Send(s string) {
	t.ptyFile.Write([]byte(s)) //nolint:errcheck
}

// SendKey sends a special key to the PTY.
func (t *Terminal) SendKey(key Key) {
	t.Send(string(key))
}

// Type sends a string character by character with a small delay between each.
func (t *Terminal) Type(s string) {
	for _, ch := range s {
		t.Send(string(ch))
		time.Sleep(10 * time.Millisecond)
	}
}

// Screenshot captures the current terminal state as plain text.
func (t *Terminal) Screenshot() Screenshot {
	// vt10x String() acquires its own internal lock
	text := t.vt.String()

	// Trim trailing blank lines but preserve internal structure
	lines := strings.Split(text, "\n")
	last := len(lines) - 1
	for last >= 0 && strings.TrimSpace(lines[last]) == "" {
		last--
	}
	if last < 0 {
		return NewScreenshot("", time.Now())
	}

	// Trim trailing spaces from each line
	trimmed := make([]string, last+1)
	for i := 0; i <= last; i++ {
		trimmed[i] = strings.TrimRight(lines[i], " ")
	}
	return NewScreenshot(strings.Join(trimmed, "\n"), time.Now())
}

// SaveScreenshot captures and saves a named screenshot to the artifacts directory.
func (t *Terminal) SaveScreenshot(name string) Screenshot {
	t.counter++
	shot := t.Screenshot()
	filename := fmt.Sprintf("%03d-%s.txt", t.counter, name)
	_ = shot.Save(filepath.Join(t.artDir, filename))
	return shot
}

// WaitFor polls until the terminal output contains substr, or times out.
// On timeout, returns an error including the last screen state.
func (t *Terminal) WaitFor(substr string, timeout time.Duration) error {
	return t.WaitForFunc(func(s Screenshot) bool {
		return s.Contains(substr)
	}, timeout)
}

// WaitForFunc polls until the predicate returns true for the current screen.
func (t *Terminal) WaitForFunc(fn func(Screenshot) bool, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			shot := t.Screenshot()
			return fmt.Errorf("timeout after %v\nlast screen:\n%s", timeout, shot.Text)
		case <-ticker.C:
			if fn(t.Screenshot()) {
				return nil
			}
		}
	}
}

// APIExchanges returns all recorded API exchanges. Returns nil if recording is disabled.
func (t *Terminal) APIExchanges() []Exchange {
	if t.recorder == nil {
		return nil
	}
	return t.recorder.Exchanges()
}

// Logs returns the current content of the application log file.
func (t *Terminal) Logs() string {
	data, err := os.ReadFile(filepath.Join(t.workDir, "aql.log"))
	if err != nil {
		return ""
	}
	return string(data)
}
