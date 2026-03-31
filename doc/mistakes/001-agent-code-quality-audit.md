# Mistake 001: Agent Code Quality Audit

**Date:** 2026-03-30
**Severity:** High (resource leak, data corruption), Medium (fragile logic, goroutine safety), Low (readability)
**Files:** `internal/agent/runner.go`, `internal/agent/tools_web.go`, `internal/agent/model.go`, `cmd/aql/main.go`

## Context

First comprehensive code audit of the agent subsystem, prompted by a checklist of 10 common AI coding agent mistakes. The codebase had been built up incrementally through TDD cycles without a dedicated quality pass.

## Issues Found and Fixed

### 1. Missing `stream.Close()` in error path (High)

**What happened:** When `stream.Err()` returned an error, `runner.go` returned immediately without closing the stream. The `stream.Close()` call on the success path was unreachable.

**Root cause:** The close was placed after the error check rather than deferred at creation time. Classic "happy path only" resource management.

**Fix:** `defer stream.Close()` immediately after `NewStreaming()`.

**Lesson:** Always defer resource cleanup at the point of acquisition. Never rely on manual close calls scattered across branches.

### 2. Map iteration order for text blocks (High)

**What happened:** `textBlocks` was a `map[int64]*strings.Builder` keyed by content block index. When building the assistant message, the code iterated `for _, sb := range textBlocks` — which in Go iterates in random order.

**Root cause:** Using a map for indexed data, then iterating it as if it were ordered. With a single text block (the common case), this was invisible.

**Fix:** Collect map keys, sort them, iterate in order.

**Lesson:** Never assume map iteration order in Go. If order matters, sort the keys or use a slice. The fact that "it usually works" (single text block) makes this worse — it only fails intermittently when there are multiple blocks.

### 3. Error strings for control flow (Medium)

**What happened:** `enrichAPIError()` used `strings.Contains(msg, "400")` and `strings.Contains(msg, "404")` to detect HTTP status codes from API errors.

**Root cause:** Quick implementation without checking what the SDK provides. The Anthropic Go SDK exports `anthropic.Error` (aliased from `internal/apierror.Error`) with a `StatusCode int` field.

**Fix:** `errors.As(err, &anthropic.Error)` + switch on `StatusCode`. Also added 403 handling which was previously missing.

**Lesson:** Before string-matching on errors, check if the library provides typed errors. Most well-maintained SDKs do. String matching is fragile — it breaks silently when error message formats change.

### 4. Goroutine not respecting context cancellation (Medium)

**What happened:** The stream consumer goroutine in `main.go` used `for evt := range ch` which only exits when the channel closes. If context was cancelled but the channel didn't close promptly, the goroutine would hang.

**Root cause:** Relying solely on upstream channel closure for goroutine lifecycle, rather than also selecting on `ctx.Done()`.

**Fix:** Changed to `select` on both `ctx.Done()` and channel receive.

**Lesson:** Goroutines that consume channels should also select on context cancellation. The channel close may come eventually, but `ctx.Done()` gives an immediate exit path. Belt and suspenders.

### 5. Hardcoded magic numbers everywhere (Low)

**What happened:** 26+ numeric literals scattered across 4 files: `25` (max iterations), `64` (channel buffer), `4096`/`16384` (max tokens), `512*1024` (body limit), `50000` (text truncation), various timeouts.

**Root cause:** Incremental development — each number was "obvious" when written, but collectively they made the code harder to reason about and tune.

**Fix:** Extracted into named `const` blocks with doc comments in each file.

**Lesson:** Extract magic numbers when:

- The same value appears in multiple places
- The value has domain meaning (max tokens, timeouts)
- Someone reading the code would ask "why this number?"

Small buffer sizes (like channel buffer `1`) in localized code are fine to leave inline.

## Issues Investigated but Not Fixed

### Goroutine lifecycle in orchestrator, auth, background probe

These were flagged but on inspection are correct:

- **orchestrator.go:** Buffered channel + `ctx.Done()` is the standard pattern. Caller owns the context.
- **auth/login.go:** Server goroutine writes to buffered(1) `errCh`; deferred `server.Shutdown()` stops it.
- **main.go background probe:** `defer bgCancel()` fires on TUI exit, cancelling the probe context.

**Lesson:** Not every goroutine without a WaitGroup is a leak. Evaluate the actual lifecycle — buffered channels, deferred cancels, and server shutdowns are valid coordination mechanisms.

### Memory scorer weights and half-life

The `0.4, 0.2, 0.4` weights and `7.0 * 24.0` half-life in `memory/scorer.go` are domain constants, not magic numbers. They're in pure functions with clear documentation and are the kind of values that should be tuned experimentally, not hidden behind const names.

## Clean Areas

- **Unused imports:** None (Go compiler enforces this)
- **Stale test fixtures:** All fixtures match current API contract
- **Race conditions:** All tests pass with `-race` flag
- **Reverted files:** `go vet` clean, no undefined references

## Prevention

1. **Resource cleanup:** Always `defer Close()` at point of acquisition
2. **Map iteration:** Grep for `range` over maps in code review; ask "does order matter?"
3. **Error handling:** Check SDK docs for typed errors before string matching
4. **Goroutine lifecycle:** Every `go func()` should have a documented exit condition
5. **Constants:** Extract when the number has domain meaning or appears more than once
