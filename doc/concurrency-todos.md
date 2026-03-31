# Concurrency Improvements

Opportunities to make better use of Go concurrency patterns across the codebase.

## High Impact

### 0. History ownership — eliminate data race on agent.history

- **Files:** `internal/agent/runner.go`, `internal/agent/agent.go`, `internal/stream/adapter.go`
- **Problem:** `Run()` goroutine mutated `a.history` concurrently with caller (auto-compact, /clear, next Run)
- **Pattern:** Run() snapshots history, works on local copy, emits `HistoryAppendMsg`/`HistoryReplaceMsg` events; caller applies via `ApplyHistory()`/`ReplaceHistory()` — single writer, no shared mutable state
- **Impact:** Eliminates data race on conversation history; aligns agent with TUI's Elm architecture (events in → state change)
- **Status:** DONE

### 1. Parallel tool execution

- **File:** `internal/agent/runner.go`
- **Problem:** When Claude returns multiple tool_use blocks (e.g. read 3 files), they execute sequentially
- **Pattern:** `errgroup` to run independent tools in parallel, collect results in order
- **Impact:** 3x faster on multi-tool turns (measured: 52ms vs 153ms for 3 tools)
- **Status:** DONE

### 2. Async event bus handlers

- **File:** `internal/events/bus.go`
- **Problem:** `Publish()` calls handlers synchronously — a slow handler blocks all others
- **Pattern:** Worker pool or per-handler goroutine with panic recovery
- **Impact:** Better responsiveness when multiple agents subscribe to the same event
- **Status:** DONE

### 3. Memory scoring worker pool

- **File:** `internal/memory/manager.go`
- **Problem:** `Query()` scores all entries sequentially, then full-sorts O(n log n) when only top-K needed
- **Pattern:** Worker pool for parallel scoring + heap for O(n log k) partial selection
- **Impact:** ~1.8x at 10k entries (scales with dataset size), 16% less memory
- **Status:** DONE

## Medium Impact

### 4. Shared memory flush lock contention

- **File:** `internal/memory/shared.go`
- **Problem:** `Flush()` holds read lock during JSON marshaling + file write
- **Pattern:** Copy entries under lock, release, then marshal/write outside the lock
- **Impact:** Reduced lock contention during concurrent reads
- **Status:** TODO

### 5. Context propagation gaps

- **Files:** `internal/memory/manager.go`, `internal/memory/shared.go`
- **Problem:** Memory operations don't accept `context.Context` — can't cancel on TUI exit
- **Pattern:** Add context parameter to `Query()`, `Flush()`, etc.
- **Impact:** Clean shutdown, no leaked goroutines
- **Status:** TODO

## Future (when multi-agent ships)

### 6. Orchestrator DAG executor

- **File:** `internal/orchestrator/orchestrator.go`
- **Problem:** No multi-agent execution logic — just a stub
- **Pattern:** DAG-based execution engine with channel plumbing between agents
- **Impact:** Enables true parallel multi-agent workflows
- **Status:** TODO
