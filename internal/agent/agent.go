package agent

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amer/aql/internal/memory"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Agent represents a single coding agent with its config, memory, and context.
type Agent struct {
	config       Config
	memManager   *memory.Manager
	claudeMD     string
	claudeMDTime time.Time // mtime of last CLAUDE.md read
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
	var claudeMDTime time.Time
	if info, err := os.Stat(filepath.Join(workDir, "CLAUDE.md")); err == nil {
		claudeMDTime = info.ModTime()
	}
	slog.Debug("loaded CLAUDE.md", "agent", cfg.Name, "length", len(claudeMD))

	memManager, err := memory.NewManager(cfg.Name, workDir)
	if err != nil {
		slog.Error("failed to init memory", "agent", cfg.Name, "error", err)
		return nil, fmt.Errorf("init memory for agent %s: %w", cfg.Name, err)
	}

	client := buildClient(o)

	a := &Agent{
		config:       cfg,
		memManager:   memManager,
		claudeMD:     claudeMD,
		claudeMDTime: claudeMDTime,
		client:       client,
		dir:          workDir,
		isOAuth:      o.isOAuth,
	}
	a.systemPrompt = BuildSystemPrompt(cfg, claudeMD, workDir)

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

// ClearHistory resets the conversation history so the next message starts
// a fresh conversation with the API — no prior context carried over.
// Called by /clear to give the user a clean slate when the conversation
// drifts off-topic or the context becomes too noisy to be useful.
func (a *Agent) ClearHistory() {
	a.history = nil
	slog.Debug("conversation history cleared", "agent", a.config.Name)
}

// HistoryLen returns the number of messages in the conversation history.
func (a *Agent) HistoryLen() int {
	return len(a.history)
}

// AppendUserMessage adds a user message to the conversation history.
func (a *Agent) AppendUserMessage(text string) {
	a.history = append(a.history, anthropic.NewUserMessage(
		anthropic.NewTextBlock(text),
	))
}

// AppendAssistantMessage adds an assistant message to the conversation history.
func (a *Agent) AppendAssistantMessage(text string) {
	a.history = append(a.history, anthropic.NewAssistantMessage(
		anthropic.NewTextBlock(text),
	))
}

// RefreshClaudeMD re-reads CLAUDE.md if it has been modified since last read.
// Returns true if the system prompt was updated.
func (a *Agent) RefreshClaudeMD() bool {
	path := filepath.Join(a.dir, "CLAUDE.md")
	info, err := os.Stat(path)
	if err != nil {
		if a.claudeMD != "" {
			a.claudeMD = ""
			a.systemPrompt = BuildSystemPrompt(a.config, "", a.dir)
			slog.Debug("CLAUDE.md removed, system prompt updated", "agent", a.config.Name)
			return true
		}
		return false
	}

	if !info.ModTime().After(a.claudeMDTime) {
		return false
	}

	content := CollectClaudeMD(a.dir)
	a.claudeMD = content
	a.claudeMDTime = info.ModTime()
	a.systemPrompt = BuildSystemPrompt(a.config, content, a.dir)
	slog.Debug("CLAUDE.md reloaded", "agent", a.config.Name, "length", len(content))
	return true
}

// ToolDescriptionsPrompt generates a tool listing from ToolDefinitions()
// so the system prompt always matches the actual registered tools.
func ToolDescriptionsPrompt() string {
	defs := ToolDefinitions()
	var b strings.Builder
	b.WriteString("# Available Tools\n\nYou have these tools available. Use the most appropriate tool for each task:\n")
	for _, d := range defs {
		b.WriteString(fmt.Sprintf("- %s: %s\n", d.Name, d.Description))
	}
	return b.String()
}

// BuildSystemPrompt constructs the system prompt from config, CLAUDE.md content,
// and environment context.
func BuildSystemPrompt(cfg Config, claudeMD string, workDir string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Role: %s", cfg.Role))
	parts = append(parts, cfg.SystemPrompt)

	// Dynamic tool descriptions from ToolDefinitions()
	parts = append(parts, ToolDescriptionsPrompt())

	// Environment context: date, platform, shell, git info
	envInfo := EnvironmentInfo(workDir, cfg.Model)
	parts = append(parts, envInfo)

	gitStatus := GitStatus(workDir)
	if gitStatus != "" {
		parts = append(parts, "# Git\n"+gitStatus)
	}

	if claudeMD != "" {
		parts = append(parts, "---\nProject context:\n"+claudeMD)
	}

	return strings.Join(parts, "\n\n")
}

// BuildSystemPromptWithMemories constructs the system prompt with injected memory context.
func BuildSystemPromptWithMemories(cfg Config, claudeMD string, workDir string, memories []string) string {
	base := BuildSystemPrompt(cfg, claudeMD, workDir)

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
