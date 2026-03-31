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
//     ensureBinary() — build-once binary compilation,
//     projectRoot() — locate the repo root.
//
// MUST NOT GO HERE:
//   - Screenshot type and methods (screenshot.go)
//   - Recorder / Exchange types (recorder.go)
//   - Terminal configuration options (option.go)
//   - Key constants (key.go)
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
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	pty "github.com/creack/pty/v2"
	"github.com/hinshun/vt10x"
)

// Terminal wraps a PTY-spawned process and a virtual terminal emulator.
type Terminal struct {
	cmd      *exec.Cmd
	ptyFile  *os.File
	vt       vt10x.Terminal
	done     chan struct{}
	cols     int
	rows     int
	artDir   string
	workDir  string
	counter  int
	recorder *Recorder // points to shared session recorder, nil if disabled
}

var (
	builtBinary string
	buildOnce   sync.Once
	buildErr    error

	sessionDir  string
	sessionOnce sync.Once

	sharedRecorder *Recorder
	sharedProxy    *httptest.Server
	recorderOnce   sync.Once
)

// sessionArtifactsDir returns the shared session directory for this test run.
// Created once per `go test` invocation, timestamped for history.
func sessionArtifactsDir() string {
	sessionOnce.Do(func() {
		ts := time.Now().Format("2006-01-02T15-04-05")
		sessionDir = filepath.Join(projectRoot(), "test", "e2e", "artifacts", ts)
	})
	return sessionDir
}

// ensureRecorder returns the session-scoped recording proxy.
// All tests in a run share one proxy so API calls are collected together.
func ensureRecorder() (*Recorder, *httptest.Server) {
	recorderOnce.Do(func() {
		upstream := "https://api.anthropic.com"
		sharedRecorder = NewRecorder(upstream)
		sharedProxy = httptest.NewServer(sharedRecorder)
	})
	return sharedRecorder, sharedProxy
}

func ensureBinary() (string, error) {
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "aql-e2e-*")
		if err != nil {
			buildErr = err
			return
		}
		builtBinary = filepath.Join(dir, "aql")
		cmd := exec.Command("go", "build", "-o", builtBinary, "./cmd/aql")
		cmd.Dir = projectRoot()
		out, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("build failed: %w\n%s", err, out)
		}
	})
	return builtBinary, buildErr
}

func projectRoot() string {
	// Walk up from this file's location to find go.mod
	dir, err := os.Getwd()
	if err != nil {
		panic("cannot get working directory: " + err.Error())
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("cannot find project root (go.mod)")
		}
		dir = parent
	}
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

	// Set up shared API recording proxy if requested
	var recorder *Recorder
	if cfg.recordAPI {
		var proxy *httptest.Server
		recorder, proxy = ensureRecorder()
		cfg.env = append(cfg.env, "ANTHROPIC_BASE_URL="+proxy.URL)
	}

	cmd := exec.Command(binary)
	cmd.Dir = cfg.workDir
	cmd.Env = append(os.Environ(), cfg.env...)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(cfg.cols),
		Rows: uint16(cfg.rows),
	})
	if err != nil {
		t.Fatalf("e2e: start pty: %v", err)
	}

	vterm := vt10x.New(vt10x.WithSize(cfg.cols, cfg.rows))

	artDir := filepath.Join(sessionArtifactsDir(), t.Name())
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		t.Fatalf("e2e: create artifacts dir: %v", err)
	}

	term := &Terminal{
		cmd:      cmd,
		ptyFile:  ptmx,
		vt:       vterm,
		done:     make(chan struct{}),
		cols:     cfg.cols,
		rows:     cfg.rows,
		artDir:   artDir,
		workDir:  cfg.workDir,
		recorder: recorder,
	}

	go term.readLoop()

	t.Cleanup(func() { term.cleanup() })

	return term
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
	// Graceful shutdown
	if t.cmd.Process != nil {
		_ = t.cmd.Process.Signal(syscall.SIGTERM)

		exited := make(chan struct{})
		go func() {
			_ = t.cmd.Wait()
			close(exited)
		}()

		select {
		case <-exited:
		case <-time.After(2 * time.Second):
			_ = t.cmd.Process.Kill()
			<-exited
		}
	}

	t.ptyFile.Close()
	<-t.done

	// Save session-level API recordings (shared across all tests)
	if t.recorder != nil {
		apiDir := filepath.Join(sessionArtifactsDir(), "api")
		_ = t.recorder.SaveExchanges(apiDir)
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
