package models

import (
	"errors"
	"net/http"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
)

func TestProbeErrorIsDefinitive(t *testing.T) {
	// Auth/not-found statuses definitively mean the account cannot use the model.
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		assert.True(t, probeErrorIsDefinitive(&anthropic.Error{StatusCode: status}), "status=%d", status)
	}

	// Transient statuses and plain errors are inconclusive, not definitive.
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		assert.False(t, probeErrorIsDefinitive(&anthropic.Error{StatusCode: status}), "status=%d", status)
	}
	assert.False(t, probeErrorIsDefinitive(errors.New("dial tcp: timeout")))
	assert.False(t, probeErrorIsDefinitive(nil))
}
