package agent

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- isRetryableError ---

func TestIsRetryableError_ServerErrors(t *testing.T) {
	tests := []struct {
		status int
		want   bool
	}{
		{500, true},
		{502, true},
		{503, true},
		{529, true},
		{400, false},
		{401, false},
		{404, false},
		{429, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.status), func(t *testing.T) {
			err := newFakeAPIError(tt.status)
			assert.Equal(t, tt.want, isRetryableError(err))
		})
	}
}

func TestIsRetryableError_StreamingErrors(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"api_error: something broke", true},
		{"overloaded_error: too busy", true},
		{"Internal server error", true},
		{"invalid_request_error: bad input", false},
		{"connection reset", false},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			err := errors.New(tt.msg)
			assert.Equal(t, tt.want, isRetryableError(err))
		})
	}
}

// --- enrichAPIError ---

func TestEnrichAPIError_400(t *testing.T) {
	err := enrichAPIError(newFakeAPIError(400), "claude-opus-4-6")
	assert.Contains(t, err.Error(), "aql auth login")
	assert.Contains(t, err.Error(), "claude-opus-4-6")
}

func TestEnrichAPIError_403(t *testing.T) {
	err := enrichAPIError(newFakeAPIError(403), "claude-opus-4-6")
	assert.Contains(t, err.Error(), "aql auth login")
	assert.Contains(t, err.Error(), "claude-opus-4-6")
}

func TestEnrichAPIError_404(t *testing.T) {
	err := enrichAPIError(newFakeAPIError(404), "claude-opus-4-6")
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "claude-opus-4-6")
}

func TestEnrichAPIError_500(t *testing.T) {
	err := enrichAPIError(newFakeAPIError(500), "claude-sonnet-4-6")
	assert.Contains(t, err.Error(), "server error")
	assert.Contains(t, err.Error(), "transient")
}

func TestEnrichAPIError_502(t *testing.T) {
	err := enrichAPIError(newFakeAPIError(502), "claude-sonnet-4-6")
	assert.Contains(t, err.Error(), "server error")
}

func TestEnrichAPIError_503(t *testing.T) {
	err := enrichAPIError(newFakeAPIError(503), "claude-sonnet-4-6")
	assert.Contains(t, err.Error(), "server error")
}

func TestEnrichAPIError_529(t *testing.T) {
	err := enrichAPIError(newFakeAPIError(529), "claude-sonnet-4-6")
	assert.Contains(t, err.Error(), "overloaded")
}

func TestEnrichAPIError_NonAPIError(t *testing.T) {
	orig := errors.New("network timeout")
	err := enrichAPIError(orig, "claude-sonnet-4-6")
	assert.Equal(t, orig, err, "non-API errors should pass through unchanged")
}

func TestEnrichAPIError_UnknownStatus(t *testing.T) {
	orig := newFakeAPIError(418)
	err := enrichAPIError(orig, "claude-sonnet-4-6")
	assert.Equal(t, orig, err, "unknown status codes should pass through unchanged")
}

// fakeAPIError implements the statusCoder interface for testing without the SDK.
type fakeAPIError struct {
	status int
	msg    string
}

func (e *fakeAPIError) Error() string {
	return fmt.Sprintf("%d: %s", e.status, e.msg)
}

func (e *fakeAPIError) StatusCode() int {
	return e.status
}

func newFakeAPIError(status int) error {
	return &fakeAPIError{status: status, msg: "test error"}
}
