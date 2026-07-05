package agent

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Spawner struct, NewSpawner() with SpawnerOption pattern
//     (WithMaxDepth, WithAgentOptions, WithToolOptions),
//     Spawn() — creates and runs child agents, depth limiting,
//     NewToolExecutor() — convenience wiring function.
//
// MUST NOT GO HERE:
//   - Tool implementations (tools/)
//   - Direct TUI interaction
//   - History from parent agent (children are isolated)
//
// Q: Should I give sub-agents access to the parent's history?
// A: No. Sub-agents get fresh conversation context. This is by design.
//
// Q: Where do I configure sub-agent tools?
// A: In Spawn(), which calls tools.NewExecutor(). Add options there.
//
// Q: How does a child inherit parent configuration (e.g. OAuth)?
// A: Via WithAgentOptions. The spawner applies those options to every
//    child it creates, recursively. Never hand-pick individual options
//    in Spawn() — that is how the sub-agent OAuth bug happened.
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/amer/aql/internal/domain"
)

const defaultMaxDepth = 3

// Spawner creates and runs child agents. It implements tools.AgentSpawner.
type Spawner struct {
	client    domain.ChatClient
	config    Config
	workDir   string
	maxDepth  int
	depth     int
	agentOpts []Option
	toolOpts  []tools.ExecutorOption
}

// SpawnerOption configures spawner creation.
type SpawnerOption func(*Spawner)

// WithMaxDepth sets the maximum nesting depth for sub-agents.
func WithMaxDepth(n int) SpawnerOption {
	return func(s *Spawner) { s.maxDepth = n }
}

// WithAgentOptions sets the agent options applied to every spawned child
// agent (e.g. WithOAuth), so children inherit the parent's configuration
// instead of silently dropping it.
func WithAgentOptions(opts ...Option) SpawnerOption {
	return func(s *Spawner) { s.agentOpts = slices.Clone(opts) }
}

// WithToolOptions sets tool-executor options applied to every child agent's
// executor (e.g. WithApprover), so sub-agents inherit the same tool policy as
// the parent instead of running side-effecting tools ungated.
func WithToolOptions(opts ...tools.ExecutorOption) SpawnerOption {
	return func(s *Spawner) { s.toolOpts = slices.Clone(opts) }
}

// NewSpawner creates a spawner that can create child agents.
func NewSpawner(client domain.ChatClient, cfg Config, workDir string, opts ...SpawnerOption) *Spawner {
	s := &Spawner{
		client:   client,
		config:   cfg,
		workDir:  workDir,
		maxDepth: defaultMaxDepth,
		depth:    0,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Spawn creates a child agent, runs the given prompt, and returns the text result.
func (s *Spawner) Spawn(ctx context.Context, prompt string) (string, error) {
	if s.depth >= s.maxDepth {
		return "", fmt.Errorf("sub-agent depth limit exceeded (max %d)", s.maxDepth)
	}

	slog.Debug("spawning sub-agent", "depth", s.depth+1, "maxDepth", s.maxDepth)

	childCfg := Config{
		Name:         fmt.Sprintf("%s-sub-%d", s.config.Name, s.depth+1),
		Role:         s.config.Role,
		SystemPrompt: s.config.SystemPrompt,
		Model:        s.config.Model,
	}

	// Child spawner at incremented depth
	childSpawner := s.child(childCfg)

	// Child executor: inherited tool policy first (e.g. WithApprover), required
	// wiring last so it always wins.
	execOpts := append(slices.Clone(s.toolOpts),
		tools.WithTaskStore(tools.NewTaskStore()),
		tools.WithAgentSpawner(childSpawner),
	)

	// Inherited options first, required wiring last so it always wins.
	childOpts := append(slices.Clone(s.agentOpts),
		WithChatClient(s.client),
		WithToolExecutor(tools.NewExecutor(execOpts...)),
	)

	childAgent, err := New(childCfg, s.workDir, childOpts...)
	if err != nil {
		return "", fmt.Errorf("create sub-agent: %w", err)
	}

	ch := childAgent.Run(ctx, prompt)

	var textParts []string
	for evt := range ch {
		if evt.Text != "" {
			textParts = append(textParts, evt.Text)
		}
		if evt.Error != nil {
			return "", fmt.Errorf("sub-agent error: %w", evt.Error)
		}
	}

	result := strings.Join(textParts, "")
	slog.Debug("sub-agent completed", "depth", s.depth+1, "resultLength", len(result))
	return result, nil
}

// child returns a spawner one nesting level deeper, inheriting the parent's
// client, workDir, depth limit, and agent options. Routing through NewSpawner
// keeps child construction in one place, so a new Spawner field can't be
// silently dropped by a hand-built literal here.
func (s *Spawner) child(cfg Config) *Spawner {
	c := NewSpawner(s.client, cfg, s.workDir,
		WithMaxDepth(s.maxDepth),
		WithAgentOptions(s.agentOpts...),
		WithToolOptions(s.toolOpts...),
	)
	c.depth = s.depth + 1
	return c
}

// NewToolExecutor creates a tool executor with full sub-agent support.
// agentOpts are inherited by every agent the spawner creates, so sub-agents
// keep the parent's configuration (e.g. OAuth billing). askFn may be nil.
func NewToolExecutor(cfg Config, client domain.ChatClient, workDir string, askFn tools.AskUserFn, agentOpts ...Option) tools.ExecutorFn {
	spawner := NewSpawner(client, cfg, workDir, WithAgentOptions(agentOpts...))
	opts := []tools.ExecutorOption{
		tools.WithTaskStore(tools.NewTaskStore()),
		tools.WithAgentSpawner(spawner),
	}
	if askFn != nil {
		opts = append(opts, tools.WithAskUser(askFn))
	}
	return tools.NewExecutor(opts...)
}
