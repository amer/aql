---
paths:
  - "internal/**"
  - "cmd/**"
---

# Architecture Rules

Unwritten rules derived from the codebase. These are patterns consistently followed across the project — not aspirational goals, but the actual constraints that keep the system coherent. Violating any of them will break something (a test, a race detector, a dependency boundary).

## Quick Reference

| #   | Rule                                                                                 | Enforced By                             |
| --- | ------------------------------------------------------------------------------------ | --------------------------------------- |
| 1   | **Dependency direction**: domain <- agent <- stream -> tui. TUI never imports agent. | Import graph                            |
| 2   | **History ownership**: Run() emits events, never writes `a.history`                  | Race detector                           |
| 3   | **Event channel protocol**: buffered, closed on exit, terminal = Done/Error          | Deadlock if violated                    |
| 4   | **Tool errors are strings**, not Go errors                                           | Runner marks `isError` wrong otherwise  |
| 5   | **Functional options** for all constructors                                          | Convention across agent, tools, spawner |
| 6   | **TUI callbacks via Set\*()**, never direct imports                                  | Dependency inversion from agent         |
| 7   | **Bubble Tea messages are value types**                                              | Immutability convention                 |
| 8   | **Two-phase transcript**: group then render                                          | ToolID matching breaks if bypassed      |
| 9   | **Three-part tool registration**: definition + handler + display mapping             | `DispatchesAllKnownTools` test          |
| 10  | **workDir threaded as parameter**, never `os.Getwd()`                                | Testability with `t.TempDir()`          |
| 11  | **Stream adapter translates, never filters**                                         | Filtering belongs in TUI handlers       |
| 12  | **External test packages**, testify, table-driven, fakes over mocks                  | Convention                              |
| 13  | **Silent tools** suppressed at earliest identification point                         | ask*user in adapter, task*\* in TUI     |

---

## 1. Dependency Direction

```
cmd/aql/main.go
    |
internal/agent      <- owns Agent, Config, Spawner
    |
internal/agent/tools <- owns ToolDef, ExecutorFn, TaskStore
    |
internal/domain      <- pure types, zero internal imports
    ^
internal/stream      <- anti-corruption layer (imports domain + tui)
    |
internal/tui         <- imports domain only, never agent
```

**Rules:**

- `domain/` imports nothing from `internal/`. It is pure type definitions — no `init()`, no global mutable state, no side effects.
- `tui/` imports `domain/` but never `agent/`. The TUI knows about messages and tool calls, not about how agents work.
- `stream/` is the only package that imports both `domain/` and `tui/`. It translates domain events into TUI messages. This is the anti-corruption layer — if either side changes shape, only the adapter changes.
- `agent/` never imports `tui/`. Communication is strictly via `domain.StreamEvent` channels.

---

## 2. History Ownership

`agent.Run()` never mutates `a.history`. It snapshots the history into a local copy at the start, works on the local copy, and emits `HistoryAppendMsg` / `HistoryReplaceMsg` events on the channel. The caller (in the `stream.ForwardWithHistory` goroutine) applies these mutations via `agent.ApplyHistory()` and `agent.ReplaceHistory()`.

**Why:** Run() executes in a goroutine. If it wrote to `a.history` directly, every read from the main goroutine (compaction, clear, model switch) would race. The event-driven design keeps all history mutation in one goroutine — no mutex needed.

**Rule:** Any new code that needs to modify history during a Run must emit a history event, never write `a.history` directly.

---

## 3. Event Channel Protocol

`agent.Run()` returns `<-chan domain.StreamEvent`. The protocol:

1. Channel is buffered (64 elements).
2. `Run()` closes the channel via `defer close(ch)` when done.
3. Events are value types with union-style fields — exactly one field is non-nil per event.
4. Terminal events: `Done: true` or `Error != nil`. After either, no more events are sent.
5. Every caller must drain the channel completely (either via `stream.Forward*` or `range ch`).

**Rule:** Never send on the channel after emitting `Done` or `Error`. Never leave a channel unconsumed.

---

## 4. Tool Error Convention

Tool handlers return `(string, error)` but the Go `error` is **never used for tool failures**. Tool execution errors (file not found, invalid input, permission denied) are returned as the string value with `nil` error:

```go
return "file not found: " + path, nil   // correct
return "", fmt.Errorf("file not found")  // wrong — breaks runner
```

The second return value (`error`) is reserved for infrastructure failures (context cancellation, unknown tool name). The runner treats a non-nil error differently from a string error — it marks the result as `isError: true` and the output as the error string.

**Rule:** Tool handlers return human-readable error messages as the first string. Only return a Go error for "this tool doesn't exist" or "context canceled" — never for domain-level failures.

---

## 5. Functional Options for Constructors

All constructors that accept configuration use the functional options pattern:

```go
agent.New(cfg, workDir, agent.WithChatClient(c), agent.WithOAuth())
tools.NewExecutor(tools.WithAskUser(fn), tools.WithTaskStore(store))
agent.NewSpawner(client, cfg, workDir, agent.WithAgentOptions(baseOpts...))
```

**Rule:** New optional dependencies get a `With*` option function. Don't add parameters to the constructor signature. Don't use config structs for optional values — the option pattern composes better and has zero-value defaults.

**Rule:** Spawners receive the parent's base agent options via `WithAgentOptions` so sub-agents inherit them (OAuth billing, etc.). Never construct a child agent from a hand-picked subset of options — that is how children silently lose configuration.

---

## 6. TUI Callback Injection

The TUI `Model` is constructed with `NewModel(name, agents, onSubmit)` — only the submit callback is required at construction. All other callbacks are injected via `Set*` methods:

```go
model.SetOnBash(onBash)
model.SetOnClear(func() { coder.ClearHistory() })
model.SetCancelStream(func() { ... })
```

**Why:** The TUI can't import `agent`. Callbacks are the dependency inversion mechanism — `main.go` closes over the agent and injects behavior the TUI needs.

**Rule:** The TUI never creates agents, calls APIs, or executes tools directly. It only knows about callbacks and message types.

---

## 7. Bubble Tea Message Types

All Bubble Tea messages (`AgentStreamDeltaMsg`, `AgentToolCallMsg`, `BashResultMsg`, etc.) are **value types**, not pointers:

```go
type AgentToolCallMsg struct {
    AgentName string
    ToolCall  domain.ToolCall  // value, not *domain.ToolCall
}
```

**Rule:** Messages are immutable data. Use value types. If a message needs a response channel (like `AgentAskUserMsg`), the channel field is the exception, not the norm.

---

## 8. Transcript Rendering Pipeline

Chat entries flow through a two-phase rendering pipeline:

1. **Grouping:** `BuildTranscriptBlocks([]ChatEntry) -> []TranscriptBlock` — flat entries become semantic blocks (user turn, assistant turn with text + tools, status).
2. **Rendering:** `RenderTranscriptBlock(block, width, expanded) -> string` — each block renders to styled terminal output.

Tool calls appear twice as `ChatEntry` items — once as `ToolRunning` (with input JSON), once as `ToolDone` (with output). `BuildTranscriptBlocks` merges them into a single `ToolEntry{Call, Result}` by matching `ToolID`.

**Rule:** New tool types that should be suppressed from the transcript (like task tools) must be filtered in `handleToolCall()` before they reach `m.chat`. The rendering pipeline doesn't filter — it renders whatever is in `m.chat`.

---

## 9. Tool Registration Pattern

Tools have three parts, kept in sync:

1. **Definition** in `Definitions()` — name, description, JSON schema. This is what the LLM sees.
2. **Handler** in `buildRegistry()` — maps tool name to `toolHandler` function.
3. **Display mapping** in `transcript.go` — `toolDisplayNames` and `toolInputExtractors` for TUI rendering.

Tools that need session state (task store, agent spawner, ask_user) are registered dynamically via `register*Tools()` functions called from `NewExecutor()`, not in `buildRegistry()`.

**Rule:** Every tool in `Definitions()` must have a handler in the registry. Every tool should have a display name mapping. The `TestDefaultExecutor_DispatchesAllKnownTools` test enforces the first invariant.

---

## 10. Working Directory Threading

The working directory is set once at `agent.New(cfg, workDir, ...)` and stored on the Agent. It flows to tools via `a.toolExecutor(ctx, a.WorkDir(), toolName, input)`. Tools resolve relative paths against it via `resolvePath(workDir, path)`.

**Rule:** Tools never call `os.Getwd()`. They always receive `workDir` as a parameter. This makes tools testable with `t.TempDir()` and ensures sub-agents can operate in different directories (e.g., git worktrees).

---

## 11. Stream Adapter Translates, Never Filters

`stream.Forward()` and `stream.ForwardWithHistory()` translate every domain event into a TUI message. They don't skip events, reorder them, or add logic. The one exception is `ask_user` — its `ToolCall`/`ToolDone` events are dropped because ask_user has its own message path (`AgentAskUserMsg`).

**Rule:** Filtering and transformation logic belongs in the TUI's `handleMsg()` / `handleToolCall()`, not in the stream adapter. The adapter is a dumb pipe with type conversion.

---

## 12. Test Conventions

- Tests use the **external test package** (`package foo_test`), testing only exported APIs. One documented exception: `styles_internal_test.go` tests unexported style helpers.
- Tests use **testify** (`assert` + `require`), not raw `if` checks.
- Tests use **table-driven** patterns when there are 3+ cases with the same structure.
- Tool tests use `t.TempDir()` for filesystem isolation — never touch the real working directory.
- Agent tests use fake `ChatClient` implementations — never hit the real API.
- The `TestDefaultExecutor_DispatchesAllKnownTools` test is the integration gate: it proves every defined tool is reachable via the executor. New tools that aren't registered will fail this test.

---

## 13. Silent Tools (Transcript Suppression)

Some tools are "internal bookkeeping" — their output is meaningful to the agent but not useful to the user as a tool call block. These are suppressed from the transcript:

- `ask_user` — suppressed in `stream/adapter.go` (has its own UX flow)
- `task_create`, `task_update`, `task_list` — suppressed in `tui/handlers.go` (routed to task panel instead)

**Rule:** Suppression happens at the earliest point where the tool can be identified. For ask_user, that's the stream adapter. For task tools, that's the TUI handler. The rendering pipeline never sees them.
