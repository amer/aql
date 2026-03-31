# Refactor Analysis: Current State

Date: 2026-03-31

## 1. What We Have (Package Map)

```
cmd/aql/main.go          — ~200 lines, wires everything together (split into setupLogging, configureTUI, startBackgroundModelProbe)
internal/
  domain/                 — pure types, zero dependencies
    types.go              — ChatClient (port), Message, ContentBlock, ChatParams, ChatResponse, StreamEvent, ToolDef
  llm/                    — Anthropic SDK adapter (implements domain.ChatClient)
    anthropic.go          — AnthropicClient, StreamMessage, SendMessage, SDK type conversions
  agent/                  — core agent logic (SDK-free)
    agent.go              — Agent struct, New(), WithChatClient(), system prompt building
    runner.go             — Run() orchestrator, streamWithRetry(), executeTools(), buildChatParamsFrom()
    compact.go            — CompactHistory, summarizeHistory, FormatHistoryForCompaction (uses domain.ChatClient)
    errors.go             — isRetryableError, enrichAPIError (SDK error inspection)
    config.go             — Config struct (YAML), MemoryConfig, EventsConfig
    context.go            — LoadClaudeMD, CollectClaudeMD
    env.go                — CheckEnv, EnvironmentInfo, GitStatus
    tools/                — tool definitions and executors (sub-package)
      defs.go             — ToolDef, Definitions(), DefaultExecutor, Execute
      tools_glob.go       — execGlob
      tools_web.go        — execWebFetch, execWebSearch, HTML parsing
  tui/                    — 27 files, ~5000 lines (UI)
    types.go              — shared enums (AgentStatus, ChatEntryType), ChatEntry, all Msg types, callbacks
    app.go                — Model struct, Update(), View(), sub-structs (streamState, paletteState, etc.)
    handlers.go           — handleKey/handleMsg/handleSubmit dispatchers + focused handler methods
    transcript.go         — tool display names, rendering
    commands.go           — slash commands, ModelTier, model picker, command palette
    streamstatus.go       — StreamPhase, FormatStreamStatus
    statusbar.go          — RenderStatusBar
    agent_panel.go        — ToolCall struct, ToolStatus, RenderToolBlock
    header.go             — RenderHeader
    input.go              — InputBuffer
    history.go            — History (input history)
    spinner.go            — SpinnerType, animations
    styles.go             — lipgloss styles
    markdown.go           — markdown rendering
    prompt.go             — prompt rendering
    welcome.go            — welcome screen
    bash.go               — bash result rendering
    clipboard.go          — clipboard
    selection.go          — mouse text selection
    cost.go               — FormatTokenCount
  events/                 — 3 files, ~170 lines
    types.go              — Event struct (Type, AgentID, Payload, Data, Timestamp)
    bus.go                — Bus (pub/sub), Subscribe, Publish, SubscribeTyped, PublishTyped
  orchestrator/           — 3 files, ~170 lines
    orchestrator.go       — Orchestrator (workflow + registry + bus)
    registry.go           — Registry (map[string]*agent.Agent)
    workflow.go            — Workflow, Execution, Pair structs (YAML)
  memory/                 — 5 files, ~300 lines
    store.go              — Entry struct, Store interface
    shortterm.go          — ShortTerm (in-memory map)
    shared.go             — Shared (JSON file on disk)
    scorer.go             — RelevanceScore, CosineSimilarity, RecencyScore
    manager.go            — Manager (coordinates short-term + shared, Query with top-K)
  auth/                   — 2 files, ~540 lines
    oauth.go              — OAuth flow, tokens
    login.go              — Login flow
```

## 2. Dependency Graph

```
cmd/aql/main.go
  ├── agent   (Config, Agent, New, Run, WithChatClient, WithOAuth)
  ├── llm     (NewAnthropicClient, WithAPIKey, WithBaseURL)
  ├── domain  (ChatClient, StreamEvent, ModelInfo)
  ├── models  (ClientConfig, FetchModels, ProbeUsableModels, ProbeAndUpdate)
  ├── auth    (LoadTokens, Login, SaveTokens)
  └── tui     (Model, NewModel, all Msg types, ModelTier)

agent
  ├── domain  (ChatClient, ChatParams, ChatResponse, Message, ContentBlock)
  ├── tools   (Definitions, ExecutorFn, AskUserFn)
  └── models  (ResolveModel)

llm (adapter)
  ├── domain  (ChatClient, ChatParams, ChatResponse — implements the port)
  └── anthropic-sdk-go (SDK types, streaming)

domain      — imports NOTHING (pure types + interfaces)
tui         — imports NOTHING from internal (standalone)
events      — imports NOTHING from internal (standalone)
memory      — imports NOTHING from internal (standalone)
auth        — imports NOTHING from internal (standalone)
```

**Key observations**:

- `tui` and `agent` never import each other. All wiring is in `main.go`.
- `agent` depends on `domain.ChatClient` (port), never on `llm` (adapter) or `anthropic-sdk-go`.
- `domain` is the innermost layer with zero dependencies.
- SDK imports are confined to: `llm/` (adapter), `agent/errors.go` (error inspection), `models/` (API probing).

## 3. Data Flow

```
User types → tui.InputBuffer → onSubmit callback (main.go)
  → agent.Run(ctx, input) returns <-chan StreamEvent
    → StreamEvent.Text        → main.go sends tui.AgentStreamDeltaMsg
    → StreamEvent.ToolCall    → main.go sends tui.AgentToolCallMsg
    → StreamEvent.ToolDone    → main.go sends tui.AgentToolCallMsg (status=Done)
    → StreamEvent.TokenUsage  → main.go sends tui.TokenUsageMsg
    → StreamEvent.Done        → main.go sends tui.AgentStreamDoneMsg
    → StreamEvent.Error       → main.go sends tui.AgentStreamErrorMsg
```

**All event translation happens in main.go's `onSubmit` goroutine** (lines 139-225).

## 4. Parallel Hierarchies Found

### 4a. Tool Call Types (agent ↔ tui)

| agent package                                      | tui package                               | Where translated |
| -------------------------------------------------- | ----------------------------------------- | ---------------- |
| `ToolCallEvent{ToolName, ToolID, Input}`           | `ToolCall{Name, Content, Status, ToolID}` | main.go:174-182  |
| `ToolDoneEvent{ToolName, ToolID, Output, IsError}` | `ToolCall{Name, Content, Status, ToolID}` | main.go:195-205  |

The TUI's `ToolCall` merges both start and done into one struct with a `Status` field.
`ToolCallEvent.Input` becomes `ToolCall.Content`, and `ToolDoneEvent.Output` also becomes `ToolCall.Content`.

### 4b. Token Usage (agent ↔ tui)

| agent package                                | tui package                                |
| -------------------------------------------- | ------------------------------------------ |
| `TokenUsageEvent{InputTokens, OutputTokens}` | `TokenUsageMsg{InputTokens, OutputTokens}` |

Identical struct, different names. Translated at main.go:207-210.

### 4c. Model Info (agent ↔ tui)

| agent package                                           | tui package                              |
| ------------------------------------------------------- | ---------------------------------------- |
| `ModelInfo{ID, DisplayName, MaxInputTokens, CreatedAt}` | `ModelTier{Label, ModelID, Description}` |

Converted by `modelsToTiers()` helper in main.go:381-391.

### 4d. Stream Events → Bubbletea Messages

The entire `StreamEvent` union type is destructured in main.go and re-packed into
separate TUI message types:

| StreamEvent field | TUI Msg type          |
| ----------------- | --------------------- |
| `.Text`           | `AgentStreamDeltaMsg` |
| `.Done`           | `AgentStreamDoneMsg`  |
| `.Error`          | `AgentStreamErrorMsg` |
| `.ToolCall`       | `AgentToolCallMsg`    |
| `.ToolDone`       | `AgentToolCallMsg`    |
| `.TokenUsage`     | `TokenUsageMsg`       |

Plus `AgentStreamStartMsg` is sent manually before `Run()`.

## 5. Unused / Underused Infrastructure

### 5a. events package — NOT USED IN PRODUCTION

The `events.Bus` is only used by `orchestrator.Orchestrator`. The orchestrator itself
is **never instantiated in main.go**. The actual app uses direct channel + goroutine
wiring. The event bus with typed generics (`SubscribeTyped`, `PublishTyped`) exists
but has no production consumers.

### 5b. orchestrator package — NOT USED IN PRODUCTION

`Orchestrator`, `Registry`, `Workflow` — none of these are imported by `main.go`.
The agent registry concept exists but main.go just holds a single `*agent.Agent` variable.
Workflow YAML configs exist in types but aren't loaded anywhere in the app.

### 5c. Config YAML fields — partially dead

`Config` has `Tools []string`, `Memory MemoryConfig`, `Events EventsConfig` fields
from YAML, but main.go constructs `Config` inline with just `Name`, `Role`,
`SystemPrompt`, `Model`. The YAML loading path (`LoadConfig`) isn't called.

### 5d. memory package — created but never queried

`memory.Manager` is created in `agent.New()` and stored as `a.memManager`. But
`Query()` is never called anywhere in production. `Memory()` accessor exists but
nothing calls it. The `Entry.Embedding` field assumes vector embeddings, but no
embedding generation exists.

### 5e. AskUserFunc global — coupling smell

`agent.AskUserFunc` is a package-level `var` set by main.go. This creates implicit
coupling: the agent package has a global mutable function pointer that main.go must
set before calling `Run()`.

## 6. The God Object: main.go

main.go does too much:

- Auth (OAuth vs API key decision)
- Model loading/caching/probing
- Agent creation
- Stream event → TUI message translation
- Bash command execution
- AskUser bridging
- Model selection callback (recreates agent)
- All callback wiring (onClear, onCompact, onModelSelected, etc.)

The `onSubmit` closure delegates to `stream.Forward()` for event translation (~10 lines).

## 7. The God Object: agent package

The `agent` package has too many responsibilities:

- **Agent lifecycle** (agent.go): creation, system prompt, memory management
- **API streaming** (runner.go): streaming loop, retry logic, tool loop
- **Model management** (model.go, model_cache.go): listing, probing, caching, resolving
- **Tool definitions and execution** (tools.go, tools_glob.go, tools_web.go): 9 tools
- **Context gathering** (context.go, env.go): CLAUDE.md, git status, env info
- **History compaction** (compact.go): summarization via API
- **Config parsing** (config.go): YAML config

These are ~7 distinct responsibilities in one package.

## 8. Summary of Smells

| Smell                       | Where                      | Impact                      |
| --------------------------- | -------------------------- | --------------------------- |
| Parallel tool call types    | agent ↔ tui                | Manual translation in main  |
| Parallel token usage types  | agent ↔ tui                | Identical struct, 2 names   |
| Dead code: events package   | events/                    | Unused in production        |
| Dead code: orchestrator     | orchestrator/              | Unused in production        |
| Dead code: memory queries   | memory.Manager.Query()     | Created, never queried      |
| Dead code: YAML config      | Config.Tools/Memory/Events | Fields never populated      |
| Global mutable: AskUserFunc | agent.AskUserFunc          | Implicit coupling           |
| God main.go                 | cmd/aql/main.go            | ~200 lines after extraction |
| God agent package           | internal/agent/            | 7+ responsibilities         |
| ModelInfo → ModelTier dance | main.go modelsToTiers()    | Could share a type          |
