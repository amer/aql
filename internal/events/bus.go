package events

import (
	"log/slog"
	"sync"
	"time"
)

// Handler is a callback invoked when an event is published.
type Handler func(Event)

type subscription struct {
	id      int
	handler Handler
}

// Bus is a publish/subscribe event bus for inter-agent communication.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[string][]subscription
	history     []Event
	nextID      int
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{
		subscribers: make(map[string][]subscription),
	}
}

// Subscribe registers a handler for the given event type.
// Returns a subscription ID for unsubscribing.
func (b *Bus) Subscribe(eventType string, h Handler) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	b.subscribers[eventType] = append(b.subscribers[eventType], subscription{
		id:      b.nextID,
		handler: h,
	})
	slog.Debug("event subscription added", "eventType", eventType, "subscriptionID", b.nextID)
	return b.nextID
}

// Unsubscribe removes a handler by subscription ID.
func (b *Bus) Unsubscribe(eventType string, id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := b.subscribers[eventType]
	for i, s := range subs {
		if s.id == id {
			b.subscribers[eventType] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

// Publish sends an event to all subscribers of that event type.
// Handlers are called synchronously in the publisher's goroutine.
func (b *Bus) Publish(e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	b.mu.Lock()
	b.history = append(b.history, e)
	subs := make([]subscription, len(b.subscribers[e.Type]))
	copy(subs, b.subscribers[e.Type])
	b.mu.Unlock()

	slog.Debug("event published", "eventType", e.Type, "agentID", e.AgentID, "subscribers", len(subs))

	for _, s := range subs {
		s.handler(e)
	}
}

// History returns past events, optionally filtered by event type.
// Pass empty string to get all events.
func (b *Bus) History(eventType string) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if eventType == "" {
		result := make([]Event, len(b.history))
		copy(result, b.history)
		return result
	}

	var result []Event
	for _, e := range b.history {
		if e.Type == eventType {
			result = append(result, e)
		}
	}
	return result
}
