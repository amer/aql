package e2e

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Exchange — captured HTTP request/response pair (value type),
//     SaveExchanges() — writes exchanges to JSON file,
//     LoadExchanges() — reads exchanges from JSON file.
//
// MUST NOT GO HERE:
//   - Recording proxy logic (recorder.go)
//   - Replay server logic (replayer.go)
//   - Terminal or screenshot types (terminal.go, screenshot.go)
//
// Q: Where do I add fields to the exchange format?
// A: Add them to the Exchange struct here. Update recorder.go's
//    recordingTransport to populate them during capture.
// ──────────────────────────────────────────────────────────────────

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Exchange represents a captured HTTP request/response pair.
// If the request was interrupted (e.g. context canceled), Error is set
// and StatusCode/ResponseBody may be empty.
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
	Error           string // non-empty if the request failed
}

// SaveExchanges writes exchanges to a JSON file in the given directory.
func SaveExchanges(dir string, exchanges []Exchange) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(exchanges, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "exchanges.json"), data, 0o644)
}

// LoadExchanges reads exchanges from a JSON file in the given directory.
func LoadExchanges(dir string) ([]Exchange, error) {
	data, err := os.ReadFile(filepath.Join(dir, "exchanges.json"))
	if err != nil {
		return nil, err
	}
	var exchanges []Exchange
	if err := json.Unmarshal(data, &exchanges); err != nil {
		return nil, err
	}
	return exchanges, nil
}
