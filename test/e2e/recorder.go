package e2e

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Recorder — reverse proxy that records all traffic,
//     NewRecorder() — creates a recording proxy for an upstream URL,
//     recordingTransport — wraps RoundTripper to capture bodies,
//     recordingBody — streams response while accumulating for recording.
//
// MUST NOT GO HERE:
//   - Exchange type and serialization (exchange.go)
//   - Replayer server (replayer.go)
//   - Terminal PTY management (terminal.go)
//
// Q: How do I record API calls?
// A: Use APIOption(fixtureDir) in scenario tests. Set E2E_RECORD=1
//    to record, omit it to replay saved fixtures.
//
// Q: Where are fixtures stored?
// A: test/e2e/testdata/<scenario>/exchanges.json — committed to git.
// ──────────────────────────────────────────────────────────────────

import (
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Recorder is an HTTP reverse proxy that captures all request/response exchanges.
// It forwards requests to an upstream server and records the traffic.
type Recorder struct {
	proxy     *httputil.ReverseProxy
	mu        sync.Mutex
	exchanges []Exchange
}

// NewRecorder creates a recording reverse proxy that forwards to upstreamURL.
func NewRecorder(upstreamURL string) *Recorder {
	target, err := url.Parse(upstreamURL)
	if err != nil {
		panic("e2e: invalid upstream URL: " + err.Error())
	}

	rec := &Recorder{}

	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}

	rec.proxy = &httputil.ReverseProxy{
		Director:      director,
		FlushInterval: -1, // flush immediately — required for SSE streaming
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			// Silently handle proxy errors (e.g. context canceled during cleanup).
			// The exchange is already recorded by recordingTransport.
			w.WriteHeader(http.StatusBadGateway)
		},
		ErrorLog: log.New(io.Discard, "", 0), // suppress default stderr logging
	}

	// Wrap the proxy transport to capture exchanges
	rec.proxy.Transport = &recordingTransport{
		inner: http.DefaultTransport,
		rec:   rec,
	}

	return rec
}

// ServeHTTP implements http.Handler — use this as the handler for httptest.NewServer.
func (r *Recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.proxy.ServeHTTP(w, req)
}

// Exchanges returns a copy of all captured exchanges.
func (r *Recorder) Exchanges() []Exchange {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Exchange, len(r.exchanges))
	copy(out, r.exchanges)
	return out
}

// SaveJSON writes all captured exchanges to a JSON file in the given directory.
func (r *Recorder) SaveJSON(dir string) error {
	return SaveExchanges(dir, r.Exchanges())
}

func (r *Recorder) record(ex Exchange) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exchanges = append(r.exchanges, ex)
}

// recordingTransport wraps an http.RoundTripper to capture request/response bodies.
type recordingTransport struct {
	inner http.RoundTripper
	rec   *Recorder
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	// Capture request body
	var reqBody string
	if req.Body != nil {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		reqBody = string(data)
		req.Body = io.NopCloser(strings.NewReader(reqBody))
	}

	// Forward request
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		// Record the failed exchange so interrupted requests aren't lost
		t.rec.record(Exchange{
			Timestamp:      start,
			Method:         req.Method,
			Path:           req.URL.Path,
			RequestHeaders: req.Header.Clone(),
			RequestBody:    reqBody,
			Duration:       time.Since(start),
			Error:          err.Error(),
		})
		return nil, err
	}

	// Wrap the response body so it streams through to the caller
	// while accumulating the full body for recording on Close().
	resp.Body = &recordingBody{
		inner: resp.Body,
		onClose: func(accumulated string) {
			t.rec.record(Exchange{
				Timestamp:       start,
				Method:          req.Method,
				Path:            req.URL.Path,
				RequestHeaders:  req.Header.Clone(),
				RequestBody:     reqBody,
				StatusCode:      resp.StatusCode,
				ResponseHeaders: resp.Header.Clone(),
				ResponseBody:    accumulated,
				Duration:        time.Since(start),
			})
		},
	}

	return resp, nil
}

// recordingBody wraps a response body, streaming bytes through to the caller
// while accumulating them. On Close(), it calls onClose with the full body.
type recordingBody struct {
	inner   io.ReadCloser
	buf     []byte
	onClose func(accumulated string)
}

func (r *recordingBody) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	if n > 0 {
		r.buf = append(r.buf, p[:n]...)
	}
	return n, err
}

func (r *recordingBody) Close() error {
	err := r.inner.Close()
	if r.onClose != nil {
		r.onClose(string(r.buf))
	}
	return err
}
