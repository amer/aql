# TUI Architecture

## Overview

The TUI is built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and
styled with [Lip Gloss](https://github.com/charmbracelet/lipgloss). Agent markdown
output is rendered with [Glamour](https://github.com/charmbracelet/glamour).

## Layout

```text
┌─────────────────────────────────────────────────┐
│ ╭─ AQL — agent orchestrator                     │  ← header.go
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
│ ┌───────────────────────────────────────────┐    │  ← command palette
│ │ ▸ /help  Show available commands          │    │    (when typing /)
│ │   /exit  Exit AQL                         │    │
│ └───────────────────────────────────────────┘    │
├─────────────────────────────────────────────────┤
│ > input text here█                               │  ← prompt.go
├─────────────────────────────────────────────────┤
│ claude-sonnet-4 · 1.5k tokens │ /exit · ctrl+c  │  ← statusbar.go
└─────────────────────────────────────────────────┘
```

## Components

| File             | Purpose                                           |
|------------------|---------------------------------------------------|
| `app.go`         | Main Bubble Tea model, Update/View, message types |
| `styles.go`      | Lip Gloss styles and color palette                |
| `header.go`      | Welcome header with branding                      |
| `statusbar.go`   | Bottom bar with model info and hints              |
| `prompt.go`      | Input prompt and streaming prompt                 |
| `spinner.go`     | Braille spinner animation                         |
| `agent_panel.go` | Agent headers, tool blocks, user messages         |
| `commands.go`    | Slash command definitions, filtering, rendering   |
| `markdown.go`    | Glamour-based markdown rendering                  |

## Data Flow

1. User types in prompt → `tea.KeyMsg` updates `Model.input`
2. User presses Enter → `SubmitFunc` called → returns `tea.Cmd`
3. Agent streams via `tea.Program.Send()` → `AgentStreamDeltaMsg` appended to chat
4. `AgentStreamDoneMsg` ends streaming state
5. `View()` renders header → chat entries → palette → prompt → status bar

## Message Types

| Message                | Trigger                         |
|------------------------|---------------------------------|
| `AgentStreamDeltaMsg`  | Each streamed text chunk        |
| `AgentStreamDoneMsg`   | Streaming complete              |
| `AgentStreamErrorMsg`  | API error during streaming      |
| `AgentOutputMsg`       | Non-streaming agent output      |
| `AgentStatusMsg`       | Agent status change             |
| `AgentToolCallMsg`     | Tool invocation by agent        |
| `SpinnerTickMsg`       | Spinner frame advance (80ms)    |
| `TokenCountMsg`        | Token count update              |

## Multiline Input

- `Alt+Enter` inserts a newline
- `Enter` submits the full input
- Input blocks while streaming (`Model.streaming`)

## Command Palette

Activated when input starts with `/`. Filters commands as the user types.
Navigation with Up/Down arrows, Tab to autocomplete, Esc to dismiss.
