package orchestrator

import (
	"context"
	"sync"

	"github.com/amer/aql/internal/agent"
	"github.com/amer/aql/internal/events"
)

// Status represents the orchestrator's lifecycle state.
type Status string

const (
	StatusIdle    Status = "idle"
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
)

// Orchestrator manages agents, their event bus, and workflow execution.
type Orchestrator struct {
	mu       sync.RWMutex
	workflow Workflow
	registry *Registry
	bus      *events.Bus
	status   Status
}

// New creates an orchestrator for the given workflow.
func New(wf Workflow) *Orchestrator {
	return &Orchestrator{
		workflow: wf,
		registry: NewRegistry(),
		bus:      events.NewBus(),
		status:   StatusIdle,
	}
}

// WorkflowName returns the name of the active workflow.
func (o *Orchestrator) WorkflowName() string {
	return o.workflow.Name
}

// RegisterAgent adds an agent to the orchestrator's registry.
func (o *Orchestrator) RegisterAgent(a *agent.Agent) {
	o.registry.Register(a)
}

// GetAgent returns an agent by name.
func (o *Orchestrator) GetAgent(name string) (*agent.Agent, bool) {
	return o.registry.Get(name)
}

// Agents returns all registered agents.
func (o *Orchestrator) Agents() []*agent.Agent {
	return o.registry.List()
}

// Bus returns the event bus.
func (o *Orchestrator) Bus() *events.Bus {
	return o.bus
}

// Status returns the current orchestrator status.
func (o *Orchestrator) Status() Status {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.status
}

func (o *Orchestrator) setStatus(s Status) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status = s
}

// Start begins workflow execution. Returns a channel that receives
// an error (or nil) when the orchestrator stops.
func (o *Orchestrator) Start(ctx context.Context) <-chan error {
	errCh := make(chan error, 1)
	o.setStatus(StatusRunning)

	go func() {
		defer func() {
			o.setStatus(StatusStopped)
		}()

		<-ctx.Done()
		errCh <- nil
	}()

	return errCh
}
