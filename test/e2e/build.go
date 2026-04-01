package e2e

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - ensureBinary() — build-once caching of the aql binary,
//     projectRoot() — locates the repo root via go.mod.
//
// MUST NOT GO HERE:
//   - Terminal spawning (terminal.go)
//   - Session/artifact management (session.go)
//   - Credential detection (credential.go)
//
// Q: Why are there package-level vars here?
// A: sync.Once ensures the binary is compiled once per test run.
//    These are write-once singletons, not general mutable state.
// ──────────────────────────────────────────────────────────────────

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

var (
	builtBinary string
	buildOnce   sync.Once
	buildErr    error
)

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
