package main

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - streamCanceller — mutex-guarded holder for the in-flight stream's
//     cancel func, shared between the Cmd goroutine that starts a stream
//     and the Update loop that cancels it.
//
// MUST NOT GO HERE:
//   - Stream forwarding or agent orchestration (internal/stream, internal/agent)
//   - Any TUI state — the TUI never sees this type
//
// Q: Why does this exist instead of a bare context.CancelFunc?
// A: The cancel func is written from a tea.Cmd goroutine (stream start) and
//    read from the Update loop goroutine (Esc). A bare shared pointer races;
//    this encapsulates the field and its mutex together.
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"sync"
)

// streamCanceller holds the cancel func for the currently-running stream.
// Set from the tea.Cmd goroutine that starts a stream; cancelActive is called
// from the Update loop goroutine when the user cancels. Access is mutex-guarded
// because those goroutines run concurrently.
type streamCanceller struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

// set records the cancel func for the stream that is starting, replacing any
// previous one.
func (s *streamCanceller) set(cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancel = cancel
}

// cancelActive cancels the in-flight stream if one is running. It is a no-op
// when no stream has started.
func (s *streamCanceller) cancelActive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}
