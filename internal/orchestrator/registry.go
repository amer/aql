package orchestrator

import (
	"sync"

	"github.com/amer/aql/internal/agent"
)

// Registry holds and manages agents by name.
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*agent.Agent
}

// NewRegistry creates an empty agent registry.
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*agent.Agent),
	}
}

// Register adds an agent to the registry.
func (r *Registry) Register(a *agent.Agent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[a.Name()] = a
}

// Get returns an agent by name.
func (r *Registry) Get(name string) (*agent.Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[name]
	return a, ok
}

// List returns all registered agents.
func (r *Registry) List() []*agent.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*agent.Agent, 0, len(r.agents))
	for _, a := range r.agents {
		result = append(result, a)
	}
	return result
}
