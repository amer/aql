package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/amer/aql/internal/domain"
)

const defaultMaxDepth = 3

// Spawner creates and runs child agents. It implements tools.AgentSpawner.
type Spawner struct {
	client   domain.ChatClient
	config   Config
	workDir  string
	maxDepth int
	depth    int
}

// SpawnerOption configures spawner creation.
type SpawnerOption func(*Spawner)

// WithMaxDepth sets the maximum nesting depth for sub-agents.
func WithMaxDepth(n int) SpawnerOption {
	return func(s *Spawner) { s.maxDepth = n }
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
	childSpawner := &Spawner{
		client:   s.client,
		config:   childCfg,
		workDir:  s.workDir,
		maxDepth: s.maxDepth,
		depth:    s.depth + 1,
	}

	childAgent, err := New(childCfg, s.workDir,
		WithChatClient(s.client),
		WithToolExecutor(tools.NewExecutor(
			tools.WithTaskStore(tools.NewTaskStore()),
			tools.WithAgentSpawner(childSpawner),
		)),
	)
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

// NewToolExecutor creates a tool executor with full sub-agent support.
// This is the convenience function used from main.go to wire everything together.
func NewToolExecutor(cfg Config, client domain.ChatClient, workDir string, askFn ...tools.AskUserFn) tools.ExecutorFn {
	spawner := NewSpawner(client, cfg, workDir)
	opts := []tools.ExecutorOption{
		tools.WithTaskStore(tools.NewTaskStore()),
		tools.WithAgentSpawner(spawner),
	}
	if len(askFn) > 0 && askFn[0] != nil {
		opts = append(opts, tools.WithAskUser(askFn[0]))
	}
	return tools.NewExecutor(opts...)
}
