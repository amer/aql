# TUI Architecture

## Overview

The TUI is built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and
styled with [Lip Gloss](https://github.com/charmbracelet/lipgloss). Agent markdown
output is rendered with [Glamour](https://github.com/charmbracelet/glamour).

## Layout

```text
┌─────────────────────────────────────────────────┐
│ ╭─ AQL — Agent Quorum Loop                      │  ← header.go
│ │ /path/to/project                              │
│ │ model: claude-sonnet-4                        │
│ ╰─                                              │
├─────────────────────────────────────────────────┤
│                                                  │
│ > user message                                   │  ← chat entries
│ ● coder                                          │    (scrolling)
│   Here is the response with **markdown**...      │
│ ╭ write_file ───────────────────────────────╮    │
│ │ ✓ internal/auth/auth.go                   │    │
│ ╰───────────────────────────────────────────╯    │
│                                                  │
├─────────────────────────────────────────────────┤
│ > input text here█                               │  ← prompt.go
│ ┌───────────────────────────────────────────┐    │  ← command palette
│ │ ▸ /help  Show available commands          │    │    (below prompt,
│ │   /exit  Exit AQL                         │    │     when typing /)
│ └───────────────────────────────────────────┘    │
├─────────────────────────────────────────────────┤
│ claude-sonnet-4 · 1.5k tokens │ /exit · ctrl+c  │  ← statusbar.go
└─────────────────────────────────────────────────┘
```

## Components

| File             | Purpose                                                                           |
| ---------------- | --------------------------------------------------------------------------------- |
| `types.go`       | Shared enums (AgentStatus, ChatEntryType), ChatEntry, all Msg types, callbacks    |
| `app.go`         | Main Bubble Tea model, Update/View, sub-structs (streamState, paletteState, etc.) |
| `handlers.go`    | Key/message handlers, submit/escape/tab dispatch                                  |
| `styles.go`      | Lip Gloss styles and color palette                                                |
| `welcome.go`     | Welcome screen with branding                                                      |
| `statusbar.go`   | Bottom bar with model name and token count                                        |
| `prompt.go`      | Input prompt and streaming prompt                                                 |
| `spinner.go`     | Spinner animations (braille, circle, arc, etc.)                                   |
| `agent_panel.go` | Agent headers, tool blocks, user messages                                         |
| `commands.go`    | Slash command definitions, filtering, rendering                                   |
| `markdown.go`    | Glamour-based markdown rendering                                                  |
| `transcript.go`  | Transcript blocks, tool grouping, search                                          |

## Data Flow

1. User types in prompt → `tea.KeyMsg` updates `Model.input`
2. User presses Enter → `SubmitFunc` called → returns `tea.Cmd`
3. `agent.Run()` starts in goroutine, emits `StreamEvent`s on channel
4. `stream.ForwardWithHistory()` translates events to TUI messages and applies history:
   - `HistoryAppendMsg` → `agent.ApplyHistory()` (caller owns history mutation)
   - `HistoryReplaceMsg` → `agent.ReplaceHistory()` (auto-compaction)
   - Text/tool/done events → `tea.Program.Send()` → TUI `Update()`
5. `AgentStreamDoneMsg` ends streaming state
6. `View()` renders welcome → chat entries → prompt → palette → status bar

## Message Types

| Message               | Trigger                      |
| --------------------- | ---------------------------- |
| `AgentStreamDeltaMsg` | Each streamed text chunk     |
| `AgentStreamDoneMsg`  | Streaming complete           |
| `AgentStreamErrorMsg` | API error during streaming   |
| `AgentOutputMsg`      | Non-streaming agent output   |
| `AgentStatusMsg`      | Agent status change          |
| `AgentToolCallMsg`    | Tool invocation by agent     |
| `SpinnerTickMsg`      | Spinner frame advance (80ms) |
| `TokenCountMsg`       | Token count update           |

## Multiline Input

- `Alt+Enter` inserts a newline
- `Enter` submits the full input
- Input blocks while streaming (`Model.streaming`)

## Command Palette

Activated when input starts with `/`. Filters commands as the user types.
Navigation with Up/Down arrows, Tab to autocomplete, Esc to dismiss.
