package e2e_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

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

	// Recorder should have captured the exchange
	exchanges := rec.Exchanges()
	require.Len(t, exchanges, 1)

	ex := exchanges[0]
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

	for i := 0; i < 3; i++ {
		resp, err := http.Get(proxy.URL + "/v1/models")
		require.NoError(t, err)
		resp.Body.Close()
	}

	assert.Len(t, rec.Exchanges(), 3)
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
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	ex := rec.Exchanges()[0]
	assert.Equal(t, "test-key", ex.RequestHeaders.Get("X-Api-Key"))
	assert.Equal(t, "abc123", ex.ResponseHeaders.Get("X-Request-Id"))
}

func TestRecorder_SaveExchanges(t *testing.T) {
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
	err = rec.SaveExchanges(dir)
	require.NoError(t, err)

	// Should have saved a file
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
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
