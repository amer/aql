package agent

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/amer/aql/internal/memory"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Agent represents a single coding agent with its config, memory, and context.
type Agent struct {
	config       Config
	memManager   *memory.Manager
	claudeMD     string
	systemPrompt string
	client       anthropic.Client
	history      []anthropic.MessageParam
	isOAuth      bool   // true when created via OAuth Console login (enables billing header for Opus)
	dir          string // working directory for tool execution
}

// New creates an agent from config. It loads CLAUDE.md from workDir
// and initializes memory.
func New(cfg Config, workDir string) (*Agent, error) {
	slog.Debug("creating agent", "agent", cfg.Name, "role", cfg.Role, "workDir", workDir)

	claudeMD := CollectClaudeMD(workDir)
	slog.Debug("loaded CLAUDE.md", "agent", cfg.Name, "length", len(claudeMD))

	memManager, err := memory.NewManager(cfg.Name, workDir)
	if err != nil {
		slog.Error("failed to init memory", "agent", cfg.Name, "error", err)
		return nil, fmt.Errorf("init memory for agent %s: %w", cfg.Name, err)
	}

	return newAgent(cfg, memManager, claudeMD, anthropic.NewClient(), workDir)
}

// NewWithOAuthKey creates an agent using an OAuth-issued API key (sk-ant-oat01-*).
// The token endpoint returns an API key, so it's sent as x-api-key, not Bearer.
func NewWithOAuthKey(cfg Config, workDir string, apiKey string, baseURL ...string) (*Agent, error) {
	slog.Debug("creating agent with OAuth key", "agent", cfg.Name, "role", cfg.Role)

	claudeMD := CollectClaudeMD(workDir)
	memManager, err := memory.NewManager(cfg.Name, workDir)
	if err != nil {
		return nil, fmt.Errorf("init memory for agent %s: %w", cfg.Name, err)
	}

	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if len(baseURL) > 0 && baseURL[0] != "" {
		opts = append(opts, option.WithBaseURL(baseURL[0]))
	}
	client := anthropic.NewClient(opts...)
	a, err := newAgent(cfg, memManager, claudeMD, client, workDir)
	if err != nil {
		return nil, err
	}
	a.isOAuth = true
	return a, nil
}

// NewWithBearerToken creates an agent that authenticates via Authorization: Bearer header.
func NewWithBearerToken(cfg Config, workDir string, token string, baseURL ...string) (*Agent, error) {
	slog.Debug("creating agent with Bearer auth", "agent", cfg.Name, "role", cfg.Role)

	claudeMD := CollectClaudeMD(workDir)
	memManager, err := memory.NewManager(cfg.Name, workDir)
	if err != nil {
		return nil, fmt.Errorf("init memory for agent %s: %w", cfg.Name, err)
	}

	opts := []option.RequestOption{option.WithAuthToken(token)}
	if len(baseURL) > 0 && baseURL[0] != "" {
		opts = append(opts, option.WithBaseURL(baseURL[0]))
	}
	client := anthropic.NewClient(opts...)
	return newAgent(cfg, memManager, claudeMD, client, workDir)
}

// NewWithBaseURL creates an agent that uses a custom API base URL.
// Useful for testing with a fake HTTP server.
func NewWithBaseURL(cfg Config, workDir string, baseURL string) (*Agent, error) {
	slog.Debug("creating agent with custom base URL", "agent", cfg.Name, "baseURL", baseURL)

	claudeMD := CollectClaudeMD(workDir)

	memManager, err := memory.NewManager(cfg.Name, workDir)
	if err != nil {
		return nil, fmt.Errorf("init memory for agent %s: %w", cfg.Name, err)
	}

	client := anthropic.NewClient(
		option.WithBaseURL(baseURL),
		option.WithAPIKey("test-key"),
	)
	return newAgent(cfg, memManager, claudeMD, client, workDir)
}

func newAgent(cfg Config, memManager *memory.Manager, claudeMD string, client anthropic.Client, workDir string) (*Agent, error) {
	a := &Agent{
		config:     cfg,
		memManager: memManager,
		claudeMD:   claudeMD,
		client:     client,
		dir:        workDir,
	}
	a.systemPrompt = BuildSystemPrompt(cfg, claudeMD)

	slog.Info("agent created", "agent", cfg.Name, "promptLength", len(a.systemPrompt))
	return a, nil
}

// Name returns the agent's name.
func (a *Agent) Name() string {
	return a.config.Name
}

// SystemPrompt returns the current system prompt.
func (a *Agent) SystemPrompt() string {
	return a.systemPrompt
}

// Memory returns the agent's memory manager.
func (a *Agent) Memory() *memory.Manager {
	return a.memManager
}

// BuildSystemPrompt constructs the system prompt from config and CLAUDE.md content.
func BuildSystemPrompt(cfg Config, claudeMD string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Role: %s", cfg.Role))
	parts = append(parts, cfg.SystemPrompt)

	if claudeMD != "" {
		parts = append(parts, "---\nProject context:\n"+claudeMD)
	}

	return strings.Join(parts, "\n\n")
}

// BuildSystemPromptWithMemories constructs the system prompt with injected memory context.
func BuildSystemPromptWithMemories(cfg Config, claudeMD string, memories []string) string {
	base := BuildSystemPrompt(cfg, claudeMD)

	if len(memories) == 0 {
		return base
	}

	var memSection strings.Builder
	memSection.WriteString("---\nRelevant context from memory:\n")
	for _, m := range memories {
		memSection.WriteString("- " + m + "\n")
	}

	return base + "\n\n" + memSection.String()
}
