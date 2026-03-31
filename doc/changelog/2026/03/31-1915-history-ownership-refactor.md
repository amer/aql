# History Ownership Refactor

**Date:** 2026-03-31

## Summary

Moved conversation history mutation out of `agent.Run()`'s goroutine and into the caller, eliminating a data race on `a.history`. Also extracted TUI shared types into `types.go` and consolidated style definitions.

## Changes

### History ownership (agent -> caller)

- `Run()` no longer mutates `a.history`. It snapshots history at call time and works on a local copy, emitting `HistoryAppendMsg` events on the channel for each message (user, assistant, tool results).
- New `domain.HistoryAppendMsg` and `domain.HistoryReplaceMsg` event types in `StreamEvent`.
- New `Agent.ApplyHistory()` and `Agent.ReplaceHistory()` methods for the caller to apply history mutations.
- `buildChatParams()` renamed to `buildChatParamsFrom()` taking explicit history parameter.
- `CompactHistory()` split: pure `summarizeHistory()` extracted (no mutation), used by `maybeAutoCompact` inside Run's goroutine.
- `maybeAutoCompact()` emits `HistoryReplaceMsg` instead of mutating `a.history`.
- New `stream.ForwardWithHistory()` applies history callbacks alongside TUI message forwarding.
- `cmd/aql/main.go` wired with `ApplyHistory`/`ReplaceHistory` callbacks.

### TUI types consolidation

- New `internal/tui/types.go`: shared enums (`AgentStatus`, `ChatEntryType`), `ChatEntry` struct, all 14 Bubbletea message types, callback type aliases (`BashFunc`, `SubmitFunc`).
- `AgentStatus` moved from `statusbar.go` (where it was misplaced) to `types.go`.
- `app.go` reduced by ~100 lines; now focused on `Model` and methods.

## Files Changed

- `internal/domain/types.go` — added `HistoryAppendMsg`, `HistoryReplaceMsg` to `StreamEvent`
- `internal/agent/runner.go` — `Run()` uses local history, emits history events
- `internal/agent/agent.go` — added `ApplyHistory()`, `ReplaceHistory()`
- `internal/agent/compact.go` — extracted `summarizeHistory()`
- `internal/agent/runner_history_test.go` — 3 new tests for history event protocol
- `internal/stream/adapter.go` — added `ForwardWithHistory()`, `HistoryCallbacks`
- `cmd/aql/main.go` — wired `ForwardWithHistory` with history callbacks
- `internal/tui/types.go` — new file with shared TUI types
- `internal/tui/app.go` — removed types now in `types.go`
- `internal/tui/statusbar.go` — removed `AgentStatus` (moved to `types.go`)
