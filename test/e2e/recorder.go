package e2e

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Exchange — captured HTTP request/response pair,
//     Recorder — reverse proxy that records all traffic,
//     NewRecorder() — creates a recording proxy for an upstream URL,
//     ServeHTTP() — implements http.Handler,
//     Exchanges() — returns captured exchanges,
//     SaveExchanges() — writes exchanges to disk,
//     recordingTransport — wraps RoundTripper to capture bodies,
//     formatExchange() / sanitizePath() — output formatting helpers.
//
// MUST NOT GO HERE:
//   - Terminal PTY management (terminal.go)
//   - Screenshot capture (screenshot.go)
//   - Terminal options (option.go)
//
// Q: How is the recorder wired into tests?
// A: Use WithRecordAPI() option on NewTerminal(). The terminal sets up
//    an httptest.Server with the recorder as handler.
//
// Q: Can I filter or modify recorded exchanges?
// A: No. The recorder captures everything verbatim. Filtering belongs
//    in test assertions.
// ──────────────────────────────────────────────────────────────────

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Exchange represents a captured HTTP request/response pair.
type Exchange struct {
	Timestamp       time.Time
	Method          string
	Path            string
	RequestHeaders  http.Header
	RequestBody     string
	StatusCode      int
	ResponseHeaders http.Header
	ResponseBody    string
	Duration        time.Duration
}

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

	rec.proxy = &httputil.ReverseProxy{Director: director}

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

// SaveExchanges writes all captured exchanges to files in the given directory.
func (r *Recorder) SaveExchanges(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	exchanges := r.Exchanges()
	for i, ex := range exchanges {
		filename := fmt.Sprintf("%03d-%s-%s.txt", i+1, ex.Method, sanitizePath(ex.Path))
		content := formatExchange(ex)
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
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
		req.Body = io.NopCloser(io.NopCloser(stringReader(reqBody)))
	}

	// Forward request
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Capture response body (read then replace)
	respData, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	respBody := string(respData)
	resp.Body = io.NopCloser(stringReader(respBody))

	t.rec.record(Exchange{
		Timestamp:       start,
		Method:          req.Method,
		Path:            req.URL.Path,
		RequestHeaders:  req.Header.Clone(),
		RequestBody:     reqBody,
		StatusCode:      resp.StatusCode,
		ResponseHeaders: resp.Header.Clone(),
		ResponseBody:    respBody,
		Duration:        time.Since(start),
	})

	return resp, nil
}

func stringReader(s string) io.Reader {
	return io.NopCloser(io.LimitReader(
		readerFunc(func(p []byte) (int, error) {
			n := copy(p, s)
			s = s[n:]
			if len(s) == 0 {
				return n, io.EOF
			}
			return n, nil
		}), int64(len(s))))
}

type readerFunc func(p []byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

func sanitizePath(path string) string {
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	result := make([]byte, 0, len(path))
	for _, c := range []byte(path) {
		if c == '/' {
			result = append(result, '-')
		} else if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		}
	}
	return string(result)
}

func formatExchange(ex Exchange) string {
	var b []byte
	b = fmt.Appendf(b, "=== REQUEST ===\n")
	b = fmt.Appendf(b, "%s %s\n", ex.Method, ex.Path)
	b = fmt.Appendf(b, "Timestamp: %s\n", ex.Timestamp.Format(time.RFC3339Nano))
	b = fmt.Appendf(b, "Duration: %s\n", ex.Duration)
	b = appendHeaders(b, ex.RequestHeaders)
	b = fmt.Appendf(b, "\n%s\n", ex.RequestBody)
	b = fmt.Appendf(b, "\n=== RESPONSE ===\n")
	b = fmt.Appendf(b, "Status: %d\n", ex.StatusCode)
	b = appendHeaders(b, ex.ResponseHeaders)
	b = fmt.Appendf(b, "\n%s\n", ex.ResponseBody)
	return string(b)
}

func appendHeaders(b []byte, h http.Header) []byte {
	for key, vals := range h {
		for _, v := range vals {
			b = fmt.Appendf(b, "%s: %s\n", key, v)
		}
	}
	return b
}
