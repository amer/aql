# System Prompt & Context Management Improvements

## Overview

Five features that give the agent better awareness of its environment,
accurate token tracking, and automatic context management.

## Changes

### 1. Environment + Git Status Injection

- System prompt now includes: date, platform, architecture, shell, CWD, model ID
- Git branch and short status injected when running inside a git repo
- New `EnvironmentInfo()` and `GitStatus()` functions in `internal/agent/env.go`
- Injected automatically via `BuildSystemPrompt()` — no manual wiring needed

### 2. Precise Token Counting

- Parse `input_tokens` and `output_tokens` from API `MessageDeltaEvent`
- New `TokenUsageEvent` emitted via the `StreamEvent` channel
- TUI receives `TokenUsageMsg` and sets exact token count (replaces char-based estimate)
- Status bar now shows real API token usage

### 3. Dynamic Tool Descriptions

- New `ToolDescriptionsPrompt()` generates tool listing from `ToolDefinitions()`
- Injected into system prompt via `BuildSystemPrompt()` automatically
- Removed hardcoded 12-line tool list from `main.go`
- Adding a new tool to `ToolDefinitions()` now auto-updates the system prompt

### 4. CLAUDE.md Hot-Reload

- New `RefreshClaudeMD()` method with mtime-based cache on `Agent`
- Called at the start of every `buildMessageParams()` (before each API call)
- If `CLAUDE.md` has been modified since last read, content is reloaded and system prompt rebuilt
- No restart required to pick up instruction changes

### 5. Auto-Compaction

- After each API response, if `input_tokens > 160,000` (80% of 200k context), auto-compact
- Calls `CompactHistory()` to summarize and replace conversation history
- Emits updated `TokenUsageEvent` so TUI reflects the reduced token count
- Prevents context window overflow without manual `/compact`

## Files Changed

- `internal/agent/env.go` — `EnvironmentInfo()`, `GitStatus()`, helpers
- `internal/agent/agent.go` — `RefreshClaudeMD()`, `ToolDescriptionsPrompt()`, updated `BuildSystemPrompt()`
- `internal/agent/runner.go` — `TokenUsageEvent`, precise token capture, auto-compact logic
- `internal/agent/compact.go` — `AutoCompactThreshold` constant
- `internal/tui/app.go` — `TokenUsageMsg` handler
- `cmd/aql/main.go` — removed hardcoded tools, wired `TokenUsage` forwarding
