package events

import "time"

// Event represents a message published on the event bus.
type Event struct {
	Type      string
	AgentID   string
	Payload   string
	Timestamp time.Time
}
