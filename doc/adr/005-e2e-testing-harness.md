# ADR-005: E2E Testing Harness via PTY

**Date:** 2026-03-31
**Status:** Accepted

## Context

The TUI has comprehensive unit and integration tests at the Bubble Tea model level
(message injection via `Update()`), but no tests that exercise the actual `aql`
binary through a real terminal. Subtle rendering bugs, alt-screen behavior, and
startup/shutdown sequences are invisible to model-level tests.

We need a way to spawn `aql` in a real terminal, interact with it as a user,
and capture what the screen looks like — without requiring a human to watch.

## Decision

Add a PTY-based e2e test harness in `test/e2e/` that:

1. Spawns the `aql` binary in a pseudo-terminal via `creack/pty`
2. Feeds PTY output to a VT10x terminal emulator (`hinshun/vt10x`) to maintain
   a 2D character grid — enabling text "screenshots"
3. Provides a `Terminal` type with methods to send keystrokes, wait for output,
   capture screenshots, and collect application logs
4. Saves artifacts (screenshots + logs) to `test/e2e/artifacts/` for offline review
5. Uses `//go:build e2e` tags so scenarios never run in the normal test suite

### Why not extend model-level tests?

Model-level tests (injecting `tea.Msg` values) are fast and deterministic but
don't exercise: binary startup, PTY initialization, alt-screen switching, real
ANSI rendering, signal handling, or log file creation. The e2e harness covers
these gaps.

### Why not a record/replay framework?

Scenario-driven Go tests are type-safe, debuggable, and follow the project's
existing test conventions. A declarative YAML format would add a parser, a runner,
and a maintenance burden for no clear benefit at this scale.

## API Recording and Replay (VCR)

Scenario tests that need the Anthropic API use a record/replay pattern:

- **Replay (default):** `APIOption(fixtureDir)` serves saved exchanges from
  `test/e2e/testdata/<scenario>/exchanges.json` via `Replayer` — no network.
- **Record:** `E2E_RECORD=1` switches to `Recorder` (reverse proxy) that
  captures live API traffic and saves fixtures on cleanup.
- Fixtures are committed to git so tests are fast and deterministic.

### Recording workflow

```sh
# Record fresh fixtures (requires ANTHROPIC_API_KEY)
E2E_RECORD=1 go test -tags e2e -v -run TestE2E_RecordAPICall -timeout 60s ./test/e2e/

# Replay (no API key needed, sub-second)
go test -tags e2e -v -run TestE2E_RecordAPICall -timeout 60s ./test/e2e/
```

### Fixture structure

```
test/e2e/testdata/
    record-api-call/exchanges.json
    edit-file-diff/exchanges.json
```

## Consequences

- Two new dependencies: `creack/pty/v2`, `hinshun/vt10x`
- E2E tests with API replay are fast (~1s); recording requires real API key
- `test/e2e/artifacts/` must be gitignored; `test/e2e/testdata/` is committed
- The harness test (`TestTerminal_SpawnAndCapture`) runs in the normal suite
  as a 2-second smoke test of the PTY plumbing

## Running

```sh
# Normal (replay mode, fast)
go test -tags e2e -v -count=1 -timeout 60s ./test/e2e/

# Record new fixtures
E2E_RECORD=1 go test -tags e2e -v -count=1 -timeout 60s ./test/e2e/
```
