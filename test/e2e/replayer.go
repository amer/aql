package e2e

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Replayer — HTTP server that replays recorded exchanges,
//     NewReplayer() — creates a replay server from exchanges.
//
// MUST NOT GO HERE:
//   - Recording proxy (recorder.go)
//   - Exchange type and serialization (exchange.go)
//
// Q: How does replay work?
// A: Exchanges are served sequentially, regardless of request path.
//    When all exchanges are exhausted, returns 502.
// ──────────────────────────────────────────────────────────────────

import (
	"io"
	"net/http"
	"sync"
)

// Replayer is an HTTP server that replays previously recorded exchanges
// in order. No network calls are made.
type Replayer struct {
	mu        sync.Mutex
	exchanges []Exchange
	index     int
}

// NewReplayer creates a replay server from a list of exchanges.
func NewReplayer(exchanges []Exchange) *Replayer {
	return &Replayer{exchanges: exchanges}
}

// ServeHTTP implements http.Handler — replays the next exchange in sequence.
func (r *Replayer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	if r.index >= len(r.exchanges) {
		r.mu.Unlock()
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	ex := r.exchanges[r.index]
	r.index++
	r.mu.Unlock()

	for key, vals := range ex.ResponseHeaders {
		for _, v := range vals {
			w.Header().Add(key, v)
		}
	}
	if ex.StatusCode != 0 {
		w.WriteHeader(ex.StatusCode)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	io.WriteString(w, ex.ResponseBody) //nolint:errcheck
}
