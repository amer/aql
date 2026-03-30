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

// Option configures agent creation.
type Option func(*agentOptions)

type agentOptions struct {
	apiKey      string
	bearerToken string
	baseURL     string
	isOAuth     bool
}

// WithAPIKey sets the API key for authentication.
func WithAPIKey(key string) Option {
	return func(o *agentOptions) { o.apiKey = key }
}

// WithOAuthKey sets an OAuth-issued API key and marks the agent as OAuth.
func WithOAuthKey(key string) Option {
	return func(o *agentOptions) { o.apiKey = key; o.isOAuth = true }
}

// WithBearerToken sets a Bearer token for authentication.
func WithBearerToken(token string) Option {
	return func(o *agentOptions) { o.bearerToken = token }
}

// WithBaseURL sets a custom API base URL (useful for testing).
func WithBaseURL(url string) Option {
	return func(o *agentOptions) { o.baseURL = url }
}

// New creates an agent from config. It loads CLAUDE.md from workDir and initializes memory.
func New(cfg Config, workDir string, opts ...Option) (*Agent, error) {
	var o agentOptions
	for _, opt := range opts {
		opt(&o)
	}

	slog.Debug("creating agent", "agent", cfg.Name, "role", cfg.Role, "workDir", workDir)

	claudeMD := CollectClaudeMD(workDir)
	slog.Debug("loaded CLAUDE.md", "agent", cfg.Name, "length", len(claudeMD))

	memManager, err := memory.NewManager(cfg.Name, workDir)
	if err != nil {
		slog.Error("failed to init memory", "agent", cfg.Name, "error", err)
		return nil, fmt.Errorf("init memory for agent %s: %w", cfg.Name, err)
	}

	client := buildClient(o)

	a := &Agent{
		config:     cfg,
		memManager: memManager,
		claudeMD:   claudeMD,
		client:     client,
		dir:        workDir,
		isOAuth:    o.isOAuth,
	}
	a.systemPrompt = BuildSystemPrompt(cfg, claudeMD)

	slog.Info("agent created", "agent", cfg.Name, "promptLength", len(a.systemPrompt))
	return a, nil
}

func buildClient(o agentOptions) anthropic.Client {
	var clientOpts []option.RequestOption

	if o.bearerToken != "" {
		clientOpts = append(clientOpts, option.WithAuthToken(o.bearerToken))
	} else if o.apiKey != "" {
		clientOpts = append(clientOpts, option.WithAPIKey(o.apiKey))
	}
	if o.baseURL != "" {
		clientOpts = append(clientOpts, option.WithBaseURL(o.baseURL))
	}

	return anthropic.NewClient(clientOpts...)
}

// NewWithOAuthKey creates an agent using an OAuth-issued API key.
// Deprecated: Use New with WithOAuthKey option instead.
func NewWithOAuthKey(cfg Config, workDir string, apiKey string, baseURL ...string) (*Agent, error) {
	opts := []Option{WithOAuthKey(apiKey)}
	if len(baseURL) > 0 && baseURL[0] != "" {
		opts = append(opts, WithBaseURL(baseURL[0]))
	}
	return New(cfg, workDir, opts...)
}

// NewWithBearerToken creates an agent that authenticates via Authorization: Bearer header.
// Deprecated: Use New with WithBearerToken option instead.
func NewWithBearerToken(cfg Config, workDir string, token string, baseURL ...string) (*Agent, error) {
	opts := []Option{WithBearerToken(token)}
	if len(baseURL) > 0 && baseURL[0] != "" {
		opts = append(opts, WithBaseURL(baseURL[0]))
	}
	return New(cfg, workDir, opts...)
}

// NewWithBaseURL creates an agent with a custom API base URL.
// Deprecated: Use New with WithBaseURL option instead.
func NewWithBaseURL(cfg Config, workDir string, baseURL string) (*Agent, error) {
	return New(cfg, workDir, WithBaseURL(baseURL), WithAPIKey("test-key"))
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
