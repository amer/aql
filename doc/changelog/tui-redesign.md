# TUI Redesign — Claude Code Style

## Overview

Complete redesign of the TUI to match Claude Code's interface style. Switched from
a dashboard panel layout to a scrolling chat interface.

## Changes

### Chat Interface

- Scrolling chat log replaces fixed agent panels
- User messages display with orange `>` prefix
- Agent responses stream inline with auto-scroll
- Multiline input via `Alt+Enter`; `Enter` submits

### Components

- **Header** (`header.go`): AQL branding with project path and model name
- **Status bar** (`statusbar.go`): Model name, formatted token count, hints
- **Spinner** (`spinner.go`): Animated braille spinner during streaming
- **Prompt** (`prompt.go`): Input prompt with cursor; streaming variant with spinner
- **Tool blocks** (`agent_panel.go`): Rounded borders with status icons (⟳/✓/✗)
- **Command palette** (`commands.go`): Slash command autocomplete popup
- **Markdown rendering** (`markdown.go`): Glamour-based styled markdown in agent output

### Slash Commands

| Command    | Action                    |
|------------|---------------------------|
| `/help`    | Show available commands   |
| `/exit`    | Exit AQL                  |
| `/quit`    | Exit AQL                  |
| `/q`       | Exit AQL                  |
| `/clear`   | Clear chat history        |
| `/agents`  | List active agents        |
| `/status`  | Show workflow status      |
| `/model`   | Show current model        |
| `/compact` | Compact context (planned) |

### Color Palette

- Brand orange: `#DA7756`
- Blue accent: `#5C94F0`
- Success green: `#5CB85C`
- Warning amber: `#D4A843`
- Error red: `#D9534F`

### Dependencies Added

- `github.com/charmbracelet/glamour` — terminal markdown rendering
