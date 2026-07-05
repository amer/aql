package e2e_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amer/aql/test/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecorder_CapturesExchange(t *testing.T) {
	// Upstream server that returns a fixed response
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"reply":"hello"}`))
	}))
	defer upstream.Close()

	rec := e2e.NewRecorder(upstream.URL)
	proxy := httptest.NewServer(rec)
	defer proxy.Close()

	// Make a request through the proxy
	resp, err := http.Post(proxy.URL+"/v1/messages", "application/json",
		strings.NewReader(`{"prompt":"hi"}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Proxy should forward the response transparently
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `{"reply":"hello"}`, string(body))

	// Recorder appends on the proxy-side body Close() (a separate goroutine),
	// so wait for the exchange to settle before inspecting it.
	require.Eventually(t, func() bool {
		return len(rec.Exchanges()) == 1
	}, 2*time.Second, 5*time.Millisecond)

	ex := rec.Exchanges()[0]
	assert.Equal(t, "POST", ex.Method)
	assert.Equal(t, "/v1/messages", ex.Path)
	assert.Equal(t, `{"prompt":"hi"}`, ex.RequestBody)
	assert.Equal(t, http.StatusOK, ex.StatusCode)
	assert.Equal(t, `{"reply":"hello"}`, ex.ResponseBody)
}

func TestRecorder_CapturesMultipleExchanges(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	rec := e2e.NewRecorder(upstream.URL)
	proxy := httptest.NewServer(rec)
	defer proxy.Close()

	for range 3 {
		resp, err := http.Get(proxy.URL + "/v1/models")
		require.NoError(t, err)
		resp.Body.Close()
	}

	// The recorder appends on the proxy-side body Close(), which the reverse
	// proxy runs on its own goroutine after streaming the response to us. So a
	// client that has already read and closed its copy can still be ahead of
	// the recording — poll until the count settles rather than racing it.
	require.Eventually(t, func() bool {
		return len(rec.Exchanges()) == 3
	}, 2*time.Second, 5*time.Millisecond)
}

func TestRecorder_CapturesHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "abc123")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	rec := e2e.NewRecorder(upstream.URL)
	proxy := httptest.NewServer(rec)
	defer proxy.Close()

	req, _ := http.NewRequest("POST", proxy.URL+"/v1/messages", strings.NewReader("body"))
	req.Header.Set("X-Api-Key", "test-key")
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	// Recording lands on the proxy-side body Close() (a separate goroutine);
	// wait for it before reading the captured headers.
	require.Eventually(t, func() bool {
		return len(rec.Exchanges()) == 1
	}, 2*time.Second, 5*time.Millisecond)

	ex := rec.Exchanges()[0]
	// Credentials must never reach committed fixtures — they are redacted at
	// capture time so an E2E_RECORD=1 run can't leak a live key into git.
	assert.Equal(t, "REDACTED", ex.RequestHeaders.Get("X-Api-Key"))
	assert.Equal(t, "REDACTED", ex.RequestHeaders.Get("Authorization"))
	// Non-sensitive headers are preserved verbatim.
	assert.Equal(t, "application/json", ex.RequestHeaders.Get("Content-Type"))
	assert.Equal(t, "abc123", ex.ResponseHeaders.Get("X-Request-Id"))
}

func TestRecorder_SaveJSON(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg_123"}`))
	}))
	defer upstream.Close()

	rec := e2e.NewRecorder(upstream.URL)
	proxy := httptest.NewServer(rec)
	defer proxy.Close()

	resp, err := http.Post(proxy.URL+"/v1/messages", "application/json",
		strings.NewReader(`{"model":"claude"}`))
	require.NoError(t, err)
	resp.Body.Close()

	dir := t.TempDir()
	err = rec.SaveJSON(dir)
	require.NoError(t, err)

	// Should have saved exchanges.json
	_, err = os.Stat(filepath.Join(dir, "exchanges.json"))
	require.NoError(t, err)
}

func TestRecorder_SSEStreamingPassthrough(t *testing.T) {
	// Simulate an SSE endpoint that streams chunks with delays.
	// The proxy must forward each chunk immediately, not buffer.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		require.True(t, ok, "ResponseWriter must support Flush")

		for i := range 3 {
			fmt.Fprintf(w, "data: chunk-%d\n\n", i)
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
		}
	}))
	defer upstream.Close()

	rec := e2e.NewRecorder(upstream.URL)
	proxy := httptest.NewServer(rec)
	defer proxy.Close()

	resp, err := http.Get(proxy.URL + "/v1/messages")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Read chunks one at a time — they should arrive as the server sends them,
	// not all at once after the stream ends.
	var chunks []string
	buf := make([]byte, 256)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			chunks = append(chunks, string(buf[:n]))
		}
		if readErr != nil {
			break
		}
	}

	// Should have received multiple reads (streaming), not one big blob
	assert.GreaterOrEqual(t, len(chunks), 2, "expected multiple streaming reads, got %d", len(chunks))

	// Full body should contain all chunks
	full := strings.Join(chunks, "")
	assert.Contains(t, full, "chunk-0")
	assert.Contains(t, full, "chunk-1")
	assert.Contains(t, full, "chunk-2")

	// Exchange should be recorded after body is fully consumed
	exchanges := rec.Exchanges()
	require.Len(t, exchanges, 1)
	assert.Contains(t, exchanges[0].ResponseBody, "chunk-0")
	assert.Contains(t, exchanges[0].ResponseBody, "chunk-2")
}

func TestRecorder_InterruptedRequest(t *testing.T) {
	// Upstream that hangs until explicitly told to stop — simulates a slow API
	hang := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-hang:
		case <-r.Context().Done():
		}
	}))
	defer upstream.Close()
	defer close(hang)

	rec := e2e.NewRecorder(upstream.URL)
	proxy := httptest.NewServer(rec)
	defer proxy.Close()

	// Send a request with a context we'll cancel
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "POST", proxy.URL+"/v1/messages",
		strings.NewReader(`{"prompt":"interrupted"}`))
	req.Header.Set("Content-Type", "application/json")

	// Start the request in a goroutine, cancel after a short delay
	done := make(chan error, 1)
	go func() {
		_, err := http.DefaultClient.Do(req) //nolint:bodyclose
		done <- err
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	err := <-done
	require.Error(t, err)

	// Give the recorder a moment to process
	time.Sleep(50 * time.Millisecond)

	// The interrupted exchange should still be recorded
	exchanges := rec.Exchanges()
	require.Len(t, exchanges, 1, "interrupted exchange should be recorded")

	ex := exchanges[0]
	assert.Equal(t, "POST", ex.Method)
	assert.Equal(t, "/v1/messages", ex.Path)
	assert.Equal(t, `{"prompt":"interrupted"}`, ex.RequestBody)
	assert.Equal(t, 0, ex.StatusCode, "no response received — status should be 0")
	assert.Contains(t, ex.Error, "canceled", "should record the error reason")
}

func TestRecorder_SaveAndLoadJSON(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg_1"}`))
	}))
	defer upstream.Close()

	rec := e2e.NewRecorder(upstream.URL)
	proxy := httptest.NewServer(rec)
	defer proxy.Close()

	resp, err := http.Post(proxy.URL+"/v1/messages", "application/json",
		strings.NewReader(`{"model":"claude"}`))
	require.NoError(t, err)
	resp.Body.Close()

	// Save to JSON
	dir := t.TempDir()
	err = rec.SaveJSON(dir)
	require.NoError(t, err)

	// Load back
	loaded, err := e2e.LoadExchanges(dir)
	require.NoError(t, err)
	require.Len(t, loaded, 1)

	ex := loaded[0]
	assert.Equal(t, "POST", ex.Method)
	assert.Equal(t, "/v1/messages", ex.Path)
	assert.Equal(t, `{"model":"claude"}`, ex.RequestBody)
	assert.Equal(t, http.StatusOK, ex.StatusCode)
	assert.Equal(t, `{"id":"msg_1"}`, ex.ResponseBody)
	assert.Equal(t, "application/json", ex.ResponseHeaders.Get("Content-Type"))
}

func TestReplayer_ServesRecordedExchanges(t *testing.T) {
	// Create exchanges to replay
	exchanges := []e2e.Exchange{
		{
			Method:          "POST",
			Path:            "/v1/messages",
			StatusCode:      http.StatusOK,
			ResponseHeaders: http.Header{"Content-Type": []string{"application/json"}},
			ResponseBody:    `{"id":"msg_1","content":[{"text":"hello"}]}`,
		},
		{
			Method:          "GET",
			Path:            "/v1/models",
			StatusCode:      http.StatusOK,
			ResponseHeaders: http.Header{"Content-Type": []string{"application/json"}},
			ResponseBody:    `{"models":[]}`,
		},
	}

	replayer := e2e.NewReplayer(exchanges)
	server := httptest.NewServer(replayer)
	defer server.Close()

	// First request — should get first exchange
	resp, err := http.Post(server.URL+"/v1/messages", "application/json",
		strings.NewReader(`{"model":"claude"}`))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `{"id":"msg_1","content":[{"text":"hello"}]}`, string(body))

	// Second request — should get second exchange
	resp, err = http.Get(server.URL + "/v1/models")
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `{"models":[]}`, string(body))
}

func TestReplayer_MatchesByMethodAndPath(t *testing.T) {
	// A chat session issues a background GET /v1/models probe concurrently with
	// the POST /v1/messages chat. The replayer must serve each request the
	// exchange recorded for its own method+path, regardless of arrival order —
	// otherwise a probe that races ahead steals the chat's response.
	exchanges := []e2e.Exchange{
		{Method: "GET", Path: "/v1/models", StatusCode: http.StatusOK, ResponseBody: `{"data":[]}`},
		{Method: "POST", Path: "/v1/messages", StatusCode: http.StatusOK, ResponseBody: `{"id":"msg_1"}`},
	}

	server := httptest.NewServer(e2e.NewReplayer(exchanges))
	defer server.Close()

	// POST arrives first (opposite of recorded order).
	resp, err := http.Post(server.URL+"/v1/messages", "application/json", strings.NewReader("{}"))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, `{"id":"msg_1"}`, string(body))

	resp, err = http.Get(server.URL + "/v1/models")
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, `{"data":[]}`, string(body))
}

func TestReplayer_Returns502WhenExhausted(t *testing.T) {
	replayer := e2e.NewReplayer([]e2e.Exchange{
		{
			Method:       "GET",
			Path:         "/v1/models",
			StatusCode:   http.StatusOK,
			ResponseBody: `{"models":[]}`,
		},
	})
	server := httptest.NewServer(replayer)
	defer server.Close()

	// First request succeeds
	resp, err := http.Get(server.URL + "/v1/models")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Second request — no more exchanges to replay
	resp, err = http.Get(server.URL + "/v1/models")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestRecorder_UpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"bad"}`))
	}))
	defer upstream.Close()

	rec := e2e.NewRecorder(upstream.URL)
	proxy := httptest.NewServer(rec)
	defer proxy.Close()

	resp, err := http.Post(proxy.URL+"/v1/messages", "application/json",
		strings.NewReader(`{"prompt":"hi"}`))
	require.NoError(t, err)
	resp.Body.Close()

	ex := rec.Exchanges()[0]
	assert.Equal(t, http.StatusInternalServerError, ex.StatusCode)
	assert.Equal(t, `{"error":"bad"}`, ex.ResponseBody)
}
