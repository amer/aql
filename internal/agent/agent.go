package agent

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/amer/aql/internal/domain"
)

// AskUserFn is the signature for a function that asks the user a question.
// Canonical definition lives in the tools sub-package.
type AskUserFn = tools.AskUserFn

// ToolExecutorFn is the signature for a function that executes a tool by name.
// Canonical definition lives in the tools sub-package.
type ToolExecutorFn = tools.ExecutorFn

// Agent represents a single coding agent with its config and context.
type Agent struct {
	config       Config
	claudeMD     string
	claudeMDTime time.Time // mtime of last CLAUDE.md read
	systemPrompt string
	chatClient   domain.ChatClient
	history      []domain.Message
	isOAuth      bool   // true when created via OAuth Console login (enables billing header for Opus)
	dir          string // working directory for tool execution
	askUser      AskUserFn
	toolExecutor ToolExecutorFn
}

// Option configures agent creation.
type Option func(*agentOptions)

type agentOptions struct {
	chatClient   domain.ChatClient
	isOAuth      bool
	askUser      AskUserFn
	toolExecutor ToolExecutorFn
}

// WithChatClient sets the ChatClient used for LLM API calls.
func WithChatClient(c domain.ChatClient) Option {
	return func(o *agentOptions) { o.chatClient = c }
}

// WithOAuth marks the agent as using OAuth billing (enables Opus/thinking).
func WithOAuth() Option {
	return func(o *agentOptions) { o.isOAuth = true }
}

// WithAskUser sets the function called when the agent uses ask_user.
func WithAskUser(fn AskUserFn) Option {
	return func(o *agentOptions) { o.askUser = fn }
}

// WithToolExecutor sets a custom tool executor (useful for testing).
func WithToolExecutor(fn ToolExecutorFn) Option {
	return func(o *agentOptions) { o.toolExecutor = fn }
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

	if o.chatClient == nil {
		return nil, fmt.Errorf("agent %q: ChatClient is required (use WithChatClient)", cfg.Name)
	}

	toolExec := o.toolExecutor
	if toolExec == nil {
		toolExec = NewToolExecutor(cfg, o.chatClient, workDir, o.askUser)
	}

	a := &Agent{
		config:       cfg,
		claudeMD:     claudeMD,
		claudeMDTime: claudeMDTime,
		chatClient:   o.chatClient,
		dir:          workDir,
		isOAuth:      o.isOAuth,
		askUser:      o.askUser,
		toolExecutor: toolExec,
	}
	a.systemPrompt = BuildSystemPrompt(cfg, claudeMD, workDir)

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
	a.history = append(a.history, domain.NewUserMessage(text))
}

// AppendAssistantMessage adds an assistant message to the conversation history.
func (a *Agent) AppendAssistantMessage(text string) {
	a.history = append(a.history, domain.NewAssistantMessage(text))
}

// ApplyHistory appends a message to the conversation history.
// Called by the caller of Run() in response to HistoryAppendMsg events,
// keeping all history mutation in the caller's goroutine.
func (a *Agent) ApplyHistory(msg domain.Message) {
	a.history = append(a.history, msg)
}

// ReplaceHistory replaces the entire conversation history.
// Called by the caller of Run() in response to HistoryReplaceMsg events
// (auto-compaction).
func (a *Agent) ReplaceHistory(msgs []domain.Message) {
	a.history = msgs
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
	defs := tools.Definitions()
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
