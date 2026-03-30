package memory

import (
	"fmt"
	"sync"
)

// ShortTerm is an in-memory store for per-agent session-scoped memory.
type ShortTerm struct {
	mu      sync.RWMutex
	entries map[string]Entry
}

// NewShortTerm creates a new short-term memory store.
func NewShortTerm() *ShortTerm {
	return &ShortTerm{
		entries: make(map[string]Entry),
	}
}

func (s *ShortTerm) Put(entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[entry.ID] = entry
	return nil
}

func (s *ShortTerm) Get(id string) (Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[id]
	if !ok {
		return Entry{}, fmt.Errorf("entry not found: %s", id)
	}
	return entry, nil
}

func (s *ShortTerm) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[id]; !ok {
		return fmt.Errorf("entry not found: %s", id)
	}
	delete(s.entries, id)
	return nil
}

func (s *ShortTerm) List(agentID string) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Entry
	for _, e := range s.entries {
		if agentID == "" || e.AgentID == agentID {
			result = append(result, e)
		}
	}
	return result, nil
}
