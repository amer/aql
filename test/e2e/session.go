package e2e

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - sessionArtifactsDir() — shared timestamped dir for this test run,
//     ensureRecorder() — session-scoped recording proxy singleton.
//
// MUST NOT GO HERE:
//   - Recorder implementation (recorder.go)
//   - Binary build caching (build.go)
//   - Terminal spawning (terminal.go)
//
// Q: Why are these singletons?
// A: All tests in a `go test` invocation share one artifacts dir
//    and one recording proxy. sync.Once ensures single initialization.
// ──────────────────────────────────────────────────────────────────

import (
	"net/http/httptest"
	"path/filepath"
	"sync"
	"time"
)

var (
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
