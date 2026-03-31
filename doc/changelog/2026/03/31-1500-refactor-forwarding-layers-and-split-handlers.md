# Delete Forwarding Layers, Add Tools Tests, Split Handlers

## Overview

Final cleanup pass: removed all forwarding layers between packages,
added direct tests for the `tools` sub-package, and split TUI message
dispatch into a dedicated `handlers.go` file.

## Changes

### 1. Delete Forwarding Layers and Deprecated Constructors

- Deleted `internal/agent/model.go` — was a 69-line forwarding layer to `internal/models/`
- Deleted `internal/agent/tools.go` — was a forwarding layer (type aliases + 4 one-liners) to `internal/agent/tools/`
- Deleted deprecated constructors from `agent.go`: `NewWithOAuthKey()`, `NewWithBearerToken()`, `NewWithBaseURL()`
- All callers now use `agent.New()` with functional options (`WithBaseURL`, `WithAPIKey`, `WithOAuthKey`)
- `compact.go` and `runner.go` now import `models` and `tools` directly
- `cmd/aql/main.go` imports `tools` directly for `tools.UserQuestion`

### 2. Add Tests for internal/agent/tools/

- Created `internal/agent/tools/tools_test.go` with 30 tests covering all 10 tools
- Tests exercise: `Definitions()`, `ToAPITools()`, `Execute()`, `DefaultExecutor()`
- Covers: read_file, write_file, edit (single/all/ambiguous/not-found/same/multiline), list_directory, bash, glob (match/recursive/no-match/hidden/sorted), grep, web_fetch (plain/html/error/invalid/scripts), web_search, ask_user (with-fn/no-fn/cancelled), unknown tool
- Deleted old `internal/agent/tools_test.go` that tested through the forwarding layer

### 3. Move Model Tests to internal/models/

- Moved `internal/agent/model_test.go` to `internal/models/model_test.go`
- Moved `internal/agent/model_cache_test.go` to `internal/models/model_cache_test.go`
- Updated test references: `agent.ResolveModel` -> `models.ResolveModel`, `agent.LoadModel` -> `models.LoadModel`
- Copied `testdata/models_list.json` to `models/testdata/`

### 4. Split app.go Message Dispatch into handlers.go

- Extracted 7 handler methods from `app.go` into `internal/tui/handlers.go` (418 lines)
- Moved: `handleModelPickerKey`, `handleKey`, `handleSubmit`, `openModelPicker`, `handleMsg`, `startStream`, `selectModel`
- `app.go` reduced from 1184 to 775 lines
- `Update()` remains in `app.go` as the thin entry point

### 5. Fix Corrupted Test Files

- Repaired 14 `agent.New()` call sites across 6 test files that were mangled by an overly greedy perl regex
- Affected files: `runner_billing_test.go`, `runner_error_test.go`, `runner_retry_test.go`, `runner_replay_test.go`, `runner_streaming_test.go`, `compact_test.go`

## Files Changed

- `internal/agent/agent.go` — removed deprecated constructors, calls `tools.DefaultExecutor()` directly
- `internal/agent/model.go` — deleted
- `internal/agent/tools.go` — deleted
- `internal/agent/tools_test.go` — deleted (replaced by tools package tests)
- `internal/agent/compact.go` — uses `models.ResolveModel()`
- `internal/agent/runner.go` — imports `tools` and `models` directly
- `internal/agent/tools/tools_test.go` — new, 30 tests
- `internal/models/model_test.go` — moved from agent, updated imports
- `internal/models/model_cache_test.go` — moved from agent, updated imports
- `internal/tui/handlers.go` — new, extracted handler methods
- `internal/tui/app.go` — reduced by ~410 lines
- `cmd/aql/main.go` — imports `tools` directly
- 6 test files — repaired corrupted `agent.New()` calls
