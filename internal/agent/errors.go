package agent

import (
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// isRetryableError returns true for transient server errors that are safe to retry.
// This includes 500 (Internal Server Error), 502, 503, 529 (Overloaded), and
// streaming errors that contain "api_error" or "overloaded_error".
func isRetryableError(err error) bool {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case 500, 502, 503, 529:
			return true
		}
		return false
	}
	// Streaming errors aren't typed — check the error message
	msg := err.Error()
	return strings.Contains(msg, "api_error") ||
		strings.Contains(msg, "overloaded_error") ||
		strings.Contains(msg, "Internal server error")
}

// enrichAPIError adds actionable context to common API errors.
// Uses the SDK's typed error to inspect HTTP status codes directly
// rather than fragile string matching.
func enrichAPIError(err error, model string) error {
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return err
	}
	switch apiErr.StatusCode {
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
