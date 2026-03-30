package events

import "time"

// Event represents a message published on the event bus.
type Event struct {
	Type      string
	AgentID   string
	Payload   string
	Data      any // typed payload for generic Subscribe/Publish
	Timestamp time.Time
}
