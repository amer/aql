# AQL Architecture Overview

## High-Level Structure

```
┌─────────────────────────────────────────────────────────────┐
│                        cmd/aql/main.go                      │
│                    (wiring & startup)                       │
│  auth.ResolveAPIKey → llm.New → agent.New → tui.New → Run   │
└──────────┬──────────────┬──────────────┬────────────────────┘
           │              │              │
           ▼              ▼              ▼
┌──────────────┐  ┌──────────────┐  ┌─────────────────────────┐
│   auth/      │  │   models/    │  │         tui/            │
│ OAuth + keys │  │ probe, save  │  │   app.go    — Model,    │
│              │  │ model picker │  │              Update/View│
└──────────────┘  └──────────────┘  │   types.go  — all msgs  │
                                    │              & ChatEntry│
                                    └────────┬────────────────┘
                                             │ onSubmit(input)
                                             ▼
                                    ┌─────────────────┐
                                    │   agent/        │
                                    │  Run() loop:    │
                                    │  API → tools →  │
                                    │  API → done     │
                                    │                 │
                                    │  Run() does not │
                                    │  mutate history │
                                    ├─────────────────┤
                                    │  agent/tools/   │
                                    │  read, write,   │
                                    │  edit, shell,   │
                                    │  glob, web,     │
                                    │  ask_user       │
                                    └───────┬─────────┘
                                            │ domain.ChatClient
                                            ▼
                                    ┌─────────────────┐
                                    │   llm/          │
                                    │  Anthropic SDK  │
                                    │  adapter        │
                                    └───────┬─────────┘
                                            │
                                            ▼
                                    ┌─────────────────┐
                                    │  Claude API     │
                                    └─────────────────┘
```

## Package Responsibilities

| Package        | Role                                                  |
| -------------- | ----------------------------------------------------- |
| `domain/`      | Pure types + interfaces (zero deps)                   |
| `llm/`         | Anthropic SDK adapter                                 |
| `agent/`       | Agentic loop: conversation, tools, retry, compaction  |
| `agent/tools/` | Tool definitions + executors (file, shell, web, glob) |
| `stream/`      | Translates domain events → TUI messages               |
| `tui/`         | Terminal UI (Bubble Tea)                              |
| `auth/`        | OAuth PKCE + API key resolution                       |
| `models/`      | Model listing, probing, persistence                   |

## Data Flow — Race-Free History

```
User types → TUI → onSubmit() → agent.Run()
                                    │
              ┌─────────────────────┘
              ▼
         Run() snapshots history into localHistory
              │
              ├── appends user msg to localHistory
              │   emits StreamEvent{History: AppendMsg}
              │
              ├── API call (using localHistory, not a.history)
              │
              ├── text response → StreamEvent{Text}
              │
              ├── tool_use → execute tools → append results to localHistory
              │   emits StreamEvent{History: AppendMsg}
              │   loop back to API
              │
              ├── auto-compact? → summarizeHistory(localHistory)
              │   emits StreamEvent{Replace: ReplaceMsgs}
              │
              └── StreamEvent{Done}
                    │
                    ▼
         stream.ForwardWithHistory()
              │
              ├── History event → calls agent.ApplyHistory(msg)
              │                   (mutates a.history in caller's goroutine)
              │
              ├── Replace event → calls agent.ReplaceHistory(msgs)
              │                   (replaces a.history in caller's goroutine)
              │
              ├── Text/ToolCall/TokenUsage → translates to TUI msg
              │
              └── send(msg) → TUI.Update() → View() → terminal
```

## History Ownership

```
┌──────────────────────────┐     ┌───────────────────────┐
│  Forward goroutine       │     │  Run() goroutine      │
│  calls ApplyHistory()    │     │  works on localHistory │
│  calls ReplaceHistory()  │     │  emits history events  │
│  (single writer)         │     │  (never touches        │
└──────────────────────────┘     │   a.history)           │
                                 └───────────────────────┘
```

`Run()` snapshots `a.history` at the start and works on `localHistory` for the
duration of the call. All mutations to the real `a.history` happen via
`ApplyHistory()` / `ReplaceHistory()`, called from `stream.ForwardWithHistory()`
in the caller's goroutine. This eliminates the data race between the streaming
goroutine and the main goroutine.

## Compaction — Pure vs Impure Split

```
CompactHistory(ctx)                       ← public, mutates a.history directly
  └── summarizeHistory(ctx, history)      ← pure, returns (summary, msgs, err)
                                            safe to call from any goroutine

maybeAutoCompact(ctx, ch, localHistory, tokens)
  └── summarizeHistory(ctx, localHistory) ← uses local copy
      emits HistoryReplaceMsg on channel  ← caller applies it
```

## StreamEvent Schema

```
StreamEvent {
    AgentName  string
    Done       bool
    Error      error
    Text       string
    ToolCall   *ToolCallEvent
    ToolDone   *ToolDoneEvent
    TokenUsage *TokenUsageEvent
    History    *HistoryAppendMsg
    Replace    *HistoryReplaceMsg
}
```

## TUI File Layout

```
tui/
├── types.go       — ChatEntry, ChatEntryType, AgentStatus,
│                    all Msg types (AgentStreamStartMsg, AgentStreamDeltaMsg,
│                    AgentStreamDoneMsg, AgentStreamErrorMsg, AgentToolCallMsg,
│                    AgentAskUserMsg, ModelSelectedMsg, ModelsLoadedMsg,
│                    BashResultMsg, CompactDoneMsg, TokenUsageMsg,
│                    BashFunc, SubmitFunc, TokenCountMsg)
├── app.go         — Model struct + Init/Update/View
├── handlers.go    — message dispatch
├── transcript.go  — chat rendering
├── statusbar.go   — bottom bar (model, tokens)
├── header.go      — title bar
├── prompt.go      — input area
├── commands.go    — slash commands + model picker
├── markdown.go    — markdown → ANSI via Glamour
├── spinner.go     — braille spinner
└── selection.go   — mouse text selection
```

## Key Design Patterns

- **Ports & Adapters** — `domain.ChatClient` interface decouples agent from Anthropic SDK
- **Event-driven streaming** — agent emits `StreamEvent` on a channel, `stream.Forward()` bridges to TUI
- **Dependency injection** — agent receives chat client, tool executor, ask-user callback
- **TUI is standalone** — imports nothing from internal, communicates via callbacks and messages
- **Functional core** — `summarizeHistory()` is pure; `CompactHistory()` applies the mutation
