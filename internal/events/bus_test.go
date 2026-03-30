package events_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/amer/aql/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBusSubscribeAndPublish(t *testing.T) {
	bus := events.NewBus()

	received := make(chan events.Event, 1)
	bus.Subscribe("code_written", func(e events.Event) {
		received <- e
	})

	bus.Publish(events.Event{
		Type:    "code_written",
		AgentID: "coder",
		Payload: "wrote auth.go",
	})

	select {
	case e := <-received:
		assert.Equal(t, "code_written", e.Type)
		assert.Equal(t, "coder", e.AgentID)
		assert.Equal(t, "wrote auth.go", e.Payload)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestBusMultipleSubscribers(t *testing.T) {
	bus := events.NewBus()

	var count int
	var mu sync.Mutex
	handler := func(e events.Event) {
		mu.Lock()
		count++
		mu.Unlock()
	}

	bus.Subscribe("code_written", handler)
	bus.Subscribe("code_written", handler)

	bus.Publish(events.Event{Type: "code_written"})

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	assert.Equal(t, 2, count)
	mu.Unlock()
}

func TestBusNoSubscribers(t *testing.T) {
	bus := events.NewBus()
	// Should not panic
	bus.Publish(events.Event{Type: "unknown"})
}

func TestBusMultipleEventTypes(t *testing.T) {
	bus := events.NewBus()

	codeEvents := make(chan events.Event, 1)
	reviewEvents := make(chan events.Event, 1)

	bus.Subscribe("code_written", func(e events.Event) { codeEvents <- e })
	bus.Subscribe("review_done", func(e events.Event) { reviewEvents <- e })

	bus.Publish(events.Event{Type: "code_written", Payload: "code"})
	bus.Publish(events.Event{Type: "review_done", Payload: "review"})

	select {
	case e := <-codeEvents:
		assert.Equal(t, "code", e.Payload)
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	select {
	case e := <-reviewEvents:
		assert.Equal(t, "review", e.Payload)
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

func TestBusConcurrentPublish(t *testing.T) {
	bus := events.NewBus()

	var count int
	var mu sync.Mutex
	bus.Subscribe("event", func(e events.Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish(events.Event{Type: "event"})
		}()
	}
	wg.Wait()

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	assert.Equal(t, 100, count)
	mu.Unlock()
}

func TestBusUnsubscribe(t *testing.T) {
	bus := events.NewBus()

	called := false
	id := bus.Subscribe("event", func(e events.Event) {
		called = true
	})

	bus.Unsubscribe("event", id)
	bus.Publish(events.Event{Type: "event"})

	time.Sleep(50 * time.Millisecond)
	assert.False(t, called)
}

func TestBusHistory(t *testing.T) {
	bus := events.NewBus()

	bus.Publish(events.Event{Type: "a", Payload: "first"})
	bus.Publish(events.Event{Type: "b", Payload: "second"})
	bus.Publish(events.Event{Type: "a", Payload: "third"})

	all := bus.History("")
	require.Len(t, all, 3)

	filtered := bus.History("a")
	require.Len(t, filtered, 2)
	assert.Equal(t, "first", filtered[0].Payload)
	assert.Equal(t, "third", filtered[1].Payload)
}

type CodeWritten struct {
	File  string
	Lines int
}

func TestSubscribeTyped(t *testing.T) {
	bus := events.NewBus()

	received := make(chan CodeWritten, 1)
	events.SubscribeTyped(bus, "code_written", func(cw CodeWritten) {
		received <- cw
	})

	events.PublishTyped(bus, "code_written", "coder", CodeWritten{File: "auth.go", Lines: 42})

	select {
	case cw := <-received:
		assert.Equal(t, "auth.go", cw.File)
		assert.Equal(t, 42, cw.Lines)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for typed event")
	}
}

func TestSubscribeTypedIgnoresWrongType(t *testing.T) {
	bus := events.NewBus()

	called := false
	events.SubscribeTyped(bus, "code_written", func(cw CodeWritten) {
		called = true
	})

	// Publish with a string payload — should be silently skipped
	bus.Publish(events.Event{Type: "code_written", Payload: "not a struct"})

	time.Sleep(50 * time.Millisecond)
	assert.False(t, called, "handler should not be called for wrong payload type")
}

func TestPublishTypedSetsData(t *testing.T) {
	bus := events.NewBus()

	events.PublishTyped(bus, "code_written", "coder", CodeWritten{File: "main.go", Lines: 10})

	history := bus.History("code_written")
	require.Len(t, history, 1)
	assert.Equal(t, "coder", history[0].AgentID)

	cw, ok := history[0].Data.(CodeWritten)
	require.True(t, ok)
	assert.Equal(t, "main.go", cw.File)
}

// --- Async publish tests ---

func TestPublishAsyncDoesNotBlockPublisher(t *testing.T) {
	bus := events.NewBus()

	bus.Subscribe("slow", func(e events.Event) {
		time.Sleep(200 * time.Millisecond) // slow handler
	})

	start := time.Now()
	bus.PublishAsync(events.Event{Type: "slow"})
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 50*time.Millisecond,
		"PublishAsync should return immediately, got %v", elapsed)

	bus.Close() // wait for handlers to finish
}

func TestPublishAsyncSlowHandlerDoesNotBlockOthers(t *testing.T) {
	bus := events.NewBus()

	var fastDone atomic.Bool

	// Slow handler
	bus.Subscribe("event", func(e events.Event) {
		time.Sleep(200 * time.Millisecond)
	})

	// Fast handler — should not wait for slow one
	bus.Subscribe("event", func(e events.Event) {
		fastDone.Store(true)
	})

	bus.PublishAsync(events.Event{Type: "event"})

	// Fast handler should complete well before slow handler
	time.Sleep(50 * time.Millisecond)
	assert.True(t, fastDone.Load(), "fast handler should complete without waiting for slow handler")

	bus.Close()
}

func TestPublishAsyncPanicRecovery(t *testing.T) {
	bus := events.NewBus()

	received := make(chan string, 1)

	// Panicking handler
	bus.Subscribe("event", func(e events.Event) {
		panic("handler exploded")
	})

	// Normal handler — should still run
	bus.Subscribe("event", func(e events.Event) {
		received <- e.Payload
	})

	bus.PublishAsync(events.Event{Type: "event", Payload: "hello"})

	select {
	case v := <-received:
		assert.Equal(t, "hello", v, "normal handler should receive event despite panic in other handler")
	case <-time.After(time.Second):
		t.Fatal("timed out — panic in one handler may have blocked others")
	}

	bus.Close()
}

func TestPublishAsyncCloseWaitsForHandlers(t *testing.T) {
	bus := events.NewBus()

	var done atomic.Bool

	bus.Subscribe("event", func(e events.Event) {
		time.Sleep(100 * time.Millisecond)
		done.Store(true)
	})

	bus.PublishAsync(events.Event{Type: "event"})

	// Close should block until handler finishes
	bus.Close()
	assert.True(t, done.Load(), "Close should wait for in-flight handlers")
}

func TestPublishAsyncConcurrentPublish(t *testing.T) {
	bus := events.NewBus()

	var count atomic.Int32
	bus.Subscribe("event", func(e events.Event) {
		count.Add(1)
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.PublishAsync(events.Event{Type: "event"})
		}()
	}
	wg.Wait()
	bus.Close()

	assert.Equal(t, int32(100), count.Load())
}

func TestPublishAsyncHistory(t *testing.T) {
	bus := events.NewBus()

	bus.PublishAsync(events.Event{Type: "a", Payload: "first"})
	bus.PublishAsync(events.Event{Type: "b", Payload: "second"})
	bus.Close()

	all := bus.History("")
	assert.Len(t, all, 2, "async published events should appear in history")
}
