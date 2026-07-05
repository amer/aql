package agent

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Agent struct, Option pattern (WithChatClient, WithOAuth, etc.),
//     New() constructor, history management (ClearHistory, ApplyHistory,
//     ReplaceHistory), system prompt assembly (BuildPromptParts,
//     JoinPromptParts, LogPromptParts), CLAUDE.md hot-reload
//     (RefreshClaudeMD), PromptPart type.
//
// MUST NOT GO HERE:
//   - Tool implementations (go to tools/)
//   - LLM API calls (go to runner.go)
//   - TUI imports or Bubble Tea types
//   - Direct history mutation from Run() goroutine (emit events instead)
//
// Q: Should I add a new agent configuration option?
// A: Add a With* functional option function and a field in agentOptions.
//
// Q: Should I mutate a.history from Run()?
// A: Never. Emit HistoryAppendMsg/HistoryReplaceMsg events. The caller
//    applies them.
//
// Q: Where do I add prompt assembly logic?
// A: Here, in BuildPromptParts(). Each section is a named PromptPart.
// ──────────────────────────────────────────────────────────────────

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/amer/aql/internal/domain"
)

// PromptPart is a named section of the system prompt.
type PromptPart struct {
	Name    string
	Content string
}

// AskUserFn is the signature for a function that asks the user a question.
// Canonical definition lives in the tools sub-package.
type AskUserFn = tools.AskUserFn

// ToolExecutorFn is the signature for a function that executes a tool by name.
// Canonical definition lives in the tools sub-package.
type ToolExecutorFn = tools.ExecutorFn

// Agent represents a single coding agent with its config and context.
//
// The mutex guards the fields that change over an agent's lifetime and may be
// read or written from more than one goroutine: history is applied from the
// stream-forwarding goroutine while /clear, /compact and model switches run on
// their own tea.Cmd goroutines; systemPrompt/claudeMD/config.Model change on
// CLAUDE.md hot-reload and model switch. Everything protected by mu must only
// be touched through the accessor methods below. Because Agent carries a mutex
// it must never be copied — always pass *Agent.
type Agent struct {
	mu           sync.Mutex
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
		// Pass this agent's own options down so spawned sub-agents
		// inherit them (OAuth billing, etc.).
		toolExec = NewToolExecutor(cfg, o.chatClient, workDir, o.askUser, opts...)
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
	parts := BuildPromptParts(cfg, claudeMD, workDir)
	a.systemPrompt = JoinPromptParts(parts)
	LogPromptParts(cfg.Name, parts)
	return a, nil
}

// Name returns the agent's name.
func (a *Agent) Name() string {
	return a.config.Name
}

// SystemPrompt returns the current system prompt.
func (a *Agent) SystemPrompt() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.systemPrompt
}

// modelName returns the configured model id under the lock.
func (a *Agent) modelName() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.config.Model
}

// SetModel switches the model the agent uses and rebuilds the system prompt
// (the environment section embeds the model name). It mutates only the guarded
// config/prompt fields, so a live Run goroutine can keep applying history
// concurrently — unlike overwriting the whole Agent value, which would copy the
// mutex and race the history slice.
func (a *Agent) SetModel(modelID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config.Model = modelID
	parts := BuildPromptParts(a.config, a.claudeMD, a.dir)
	a.systemPrompt = JoinPromptParts(parts)
	LogPromptParts(a.config.Name, parts)
	slog.Info("model switched", "agent", a.config.Name, "model", modelID)
}

// ClearHistory resets the conversation history so the next message starts
// a fresh conversation with the API — no prior context carried over.
// Called by /clear to give the user a clean slate when the conversation
// drifts off-topic or the context becomes too noisy to be useful.
func (a *Agent) ClearHistory() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = nil
	slog.Debug("conversation history cleared", "agent", a.config.Name)
}

// HistoryLen returns the number of messages in the conversation history.
func (a *Agent) HistoryLen() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.history)
}

// snapshotHistory returns a copy of the current history for a Run goroutine to
// work on without touching shared state after the snapshot is taken.
func (a *Agent) snapshotHistory() []domain.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]domain.Message, len(a.history))
	copy(out, a.history)
	return out
}

// ApplyHistory appends a message to the conversation history.
// Called by the caller of Run() in response to HistoryAppendMsg events,
// keeping all history mutation in the caller's goroutine.
func (a *Agent) ApplyHistory(msg domain.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = append(a.history, msg)
}

// ReplaceHistory replaces the entire conversation history.
// Called by the caller of Run() in response to HistoryReplaceMsg events
// (auto-compaction).
func (a *Agent) ReplaceHistory(msgs []domain.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = msgs
}

// RefreshClaudeMD re-reads CLAUDE.md if it has been modified since last read.
// Returns true if the system prompt was updated.
func (a *Agent) RefreshClaudeMD() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	path := filepath.Join(a.dir, "CLAUDE.md")
	info, err := os.Stat(path)
	if err != nil {
		if a.claudeMD != "" {
			a.claudeMD = ""
			parts := BuildPromptParts(a.config, "", a.dir)
			a.systemPrompt = JoinPromptParts(parts)
			LogPromptParts(a.config.Name, parts)
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
	parts := BuildPromptParts(a.config, content, a.dir)
	a.systemPrompt = JoinPromptParts(parts)
	LogPromptParts(a.config.Name, parts)
	return true
}

// BuildPromptParts constructs the system prompt as named parts.
func BuildPromptParts(cfg Config, claudeMD string, workDir string) []PromptPart {
	var parts []PromptPart

	parts = append(parts, PromptPart{Name: "role", Content: fmt.Sprintf("Role: %s", cfg.Role)})
	parts = append(parts, PromptPart{Name: "system", Content: cfg.SystemPrompt})

	envInfo := EnvironmentInfo(workDir, cfg.Model)
	parts = append(parts, PromptPart{Name: "environment", Content: envInfo})

	if gitStatus := GitStatus(workDir); gitStatus != "" {
		parts = append(parts, PromptPart{Name: "git", Content: "# Git\n" + gitStatus})
	}

	if claudeMD != "" {
		parts = append(parts, PromptPart{Name: "project-context", Content: "---\nProject context:\n" + claudeMD})
	}

	return parts
}

// JoinPromptParts concatenates prompt part contents with double newlines.
func JoinPromptParts(parts []PromptPart) string {
	strs := make([]string, len(parts))
	for i, p := range parts {
		strs[i] = p.Content
	}
	return strings.Join(strs, "\n\n")
}

// LogPromptParts logs each prompt part's name and size at Debug level,
// and the total at Info level.
func LogPromptParts(agentName string, parts []PromptPart) {
	total := 0
	for _, p := range parts {
		slog.Debug("prompt part", "agent", agentName, "part", p.Name, "chars", len(p.Content))
		total += len(p.Content)
	}
	slog.Info("system prompt assembled", "agent", agentName, "parts", len(parts), "totalChars", total)
}

// BuildSystemPrompt constructs the system prompt as a single string.
// Delegates to BuildPromptParts + JoinPromptParts.
func BuildSystemPrompt(cfg Config, claudeMD string, workDir string) string {
	return JoinPromptParts(BuildPromptParts(cfg, claudeMD, workDir))
}
