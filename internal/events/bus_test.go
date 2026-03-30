package events_test

import (
	"sync"
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
