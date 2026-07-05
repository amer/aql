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
// A: Exchanges are keyed by "METHOD PATH" and served in recorded order within
//    each key. This keeps replay deterministic when a background GET /v1/models
//    probe races the POST /v1/messages chat. Exhausted or unknown keys 502.
// ──────────────────────────────────────────────────────────────────

import (
	"io"
	"net/http"
	"sync"
)

// Replayer is an HTTP server that replays previously recorded exchanges,
// matching each request to the exchange recorded for its method and path.
// No network calls are made.
type Replayer struct {
	mu     sync.Mutex
	queues map[string][]Exchange
}

// NewReplayer creates a replay server from a list of exchanges.
func NewReplayer(exchanges []Exchange) *Replayer {
	queues := make(map[string][]Exchange)
	for _, ex := range exchanges {
		key := ex.Method + " " + ex.Path
		queues[key] = append(queues[key], ex)
	}
	return &Replayer{queues: queues}
}

// ServeHTTP implements http.Handler — replays the next exchange recorded for
// the request's method and path.
func (r *Replayer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	key := req.Method + " " + req.URL.Path

	r.mu.Lock()
	queue := r.queues[key]
	if len(queue) == 0 {
		r.mu.Unlock()
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	ex := queue[0]
	r.queues[key] = queue[1:]
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
