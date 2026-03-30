package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Shared is a cross-agent memory store backed by a JSON file on disk.
type Shared struct {
	mu      sync.RWMutex
	entries map[string]Entry
	path    string
}

// NewShared creates a shared memory store. If the file at path exists,
// entries are loaded from it. Otherwise starts empty.
func NewShared(path string) (*Shared, error) {
	s := &Shared{
		entries: make(map[string]Entry),
		path:    path,
	}

	data, err := os.ReadFile(path)
	if err == nil {
		var entries []Entry
		if err := json.Unmarshal(data, &entries); err != nil {
			return nil, fmt.Errorf("corrupt shared memory file: %w", err)
		}
		for _, e := range entries {
			s.entries[e.ID] = e
		}
	}

	return s, nil
}

func (s *Shared) Put(entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[entry.ID] = entry
	return nil
}

func (s *Shared) Get(id string) (Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[id]
	if !ok {
		return Entry{}, fmt.Errorf("entry not found: %s", id)
	}
	return entry, nil
}

func (s *Shared) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[id]; !ok {
		return fmt.Errorf("entry not found: %s", id)
	}
	delete(s.entries, id)
	return nil
}

func (s *Shared) List(agentID string) ([]Entry, error) {
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

// Flush writes all entries to disk.
func (s *Shared) Flush() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entries []Entry
	for _, e := range s.entries {
		entries = append(entries, e)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal shared memory: %w", err)
	}

	return os.WriteFile(s.path, data, 0644)
}
