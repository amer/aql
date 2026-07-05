package main

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStreamCanceller_CancelActiveInvokesStoredCancel(t *testing.T) {
	var c streamCanceller
	called := false
	c.set(func() { called = true })

	c.cancelActive()

	assert.True(t, called, "cancelActive should invoke the stored cancel func")
}

func TestStreamCanceller_CancelActiveWithNilIsNoOp(t *testing.T) {
	var c streamCanceller
	assert.NotPanics(t, func() { c.cancelActive() })
}

func TestStreamCanceller_ConcurrentSetAndCancelIsRaceFree(t *testing.T) {
	// Mirrors production: one goroutine sets the cancel func as a stream starts
	// while another cancels it. Run under -race to catch unsynchronized access.
	var c streamCanceller
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(2)
		go func() { defer wg.Done(); c.set(func() {}) }()
		go func() { defer wg.Done(); c.cancelActive() }()
	}
	wg.Wait()
}
