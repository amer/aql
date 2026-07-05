package agent

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - statusCoder interface, extractStatusCode(), isRetryableError(),
//     enrichAPIError() — all pure functions for error classification
//     and enrichment.
//
// MUST NOT GO HERE:
//   - Retry logic itself (that's runner.go's streamWithRetry)
//   - Anthropic SDK types (uses interface, not concrete)
//   - Error creation
//
// Q: Should I add a new retryable error code?
// A: Add it to the switch in isRetryableError().
//
// Q: Should I add user-facing error messages?
// A: Add a case to enrichAPIError(). It wraps errors with actionable
//    context.
// ──────────────────────────────────────────────────────────────────

import (
	"errors"
	"fmt"
	"strings"
)

// statusCoder is satisfied by any error that carries an HTTP status code.
// Both *anthropic.Error and test fakes implement this, decoupling error
// inspection from the SDK.
type statusCoder interface {
	error
	StatusCode() int
}

// extractStatusCode returns the HTTP status code from an error if it
// implements statusCoder, and -1 otherwise.
func extractStatusCode(err error) int {
	var sc statusCoder
	if errors.As(err, &sc) {
		return sc.StatusCode()
	}
	return -1
}

// isRetryableError returns true for transient server errors that are safe to retry.
// This includes 500 (Internal Server Error), 502, 503, 529 (Overloaded), and
// streaming errors that contain "api_error" or "overloaded_error".
func isRetryableError(err error) bool {
	if code := extractStatusCode(err); code > 0 {
		switch code {
		case 500, 502, 503, 529:
			return true
		}
		return false
	}
	// Streaming (SSE) errors arrive as an `event: error` payload, not an HTTP
	// response, so they carry no status code to switch on. The only signal we
	// have is the error text, which embeds the API's stable error `type`
	// (api_error, overloaded_error) or its message. Substring matching is
	// deliberate here — the alternative is plumbing a typed error up from the
	// SSE decoder, which isn't worth it for these fixed contract strings.
	msg := err.Error()
	return strings.Contains(msg, "api_error") ||
		strings.Contains(msg, "overloaded_error") ||
		strings.Contains(msg, "Internal server error")
}

// enrichAPIError adds actionable context to common API errors.
func enrichAPIError(err error, model string) error {
	code := extractStatusCode(err)
	if code < 0 {
		return err
	}
	switch code {
	case 400, 403:
		return fmt.Errorf("%w — your API key may not have access to %s. "+
			"Run `aql auth login --console` for full model access, "+
			"or /model to switch models", err, model)
	case 404:
		return fmt.Errorf("%w — model %q not found. Try /model to pick a valid model", err, model)
	case 500, 502, 503:
		return fmt.Errorf("%w — API server error. This is usually transient, try again", err)
	case 529:
		return fmt.Errorf("%w — API is overloaded, try again in a moment", err)
	default:
		return err
	}
}
