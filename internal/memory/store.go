package memory

import "time"

// Entry represents a single memory entry across all layers.
type Entry struct {
	ID          string
	AgentID     string
	Content     string
	Tags        []string
	CreatedAt   time.Time
	LastAccess  time.Time
	AccessCount int
	Embedding   []float32
}

// Store is the common interface for all memory layers.
type Store interface {
	// Put stores a memory entry.
	Put(entry Entry) error

	// Get retrieves a memory entry by ID.
	Get(id string) (Entry, error)

	// Delete removes a memory entry by ID.
	Delete(id string) error

	// List returns all entries, optionally filtered by agent ID.
	List(agentID string) ([]Entry, error)
}
