# Mistake 002: E2E Tests Blocked 5s by termenv OSC Query

**Date:** 2026-04-01
**Severity:** Medium (test performance — every e2e test took 5s longer than necessary)
**Files:** `test/e2e/terminal.go`

## Context

Every PTY-based e2e test was taking ~5 seconds regardless of complexity. A test that just waited for the welcome screen to render took 5.8s. The model probe was suspected but ruled out quickly (completes in ~30ms against the stub API).

## Root Cause

Bubble Tea's `init()` function (in `bubbletea@v1.3.10/tea_init.go`) calls `lipgloss.HasDarkBackground()` at package load time. This sends an OSC (Operating System Command) escape sequence to query the terminal's background color and waits for a response.

The chain:

1. `aql` binary starts → Go loads the `bubbletea` package
2. `bubbletea/tea_init.go:init()` calls `lipgloss.HasDarkBackground()`
3. lipgloss calls `termenv.BackgroundColor()` which writes an OSC query to stdout
4. termenv waits for the terminal to respond with its background color
5. **vt10x** (the virtual terminal emulator used in e2e tests) doesn't handle OSC queries — it never responds
6. termenv blocks for `OSCTimeout` = **5 seconds** (`termenv@v0.16.0/termenv_unix.go`)
7. Only after the timeout does the Bubble Tea program proceed to render

This is a known workaround in Bubble Tea (the comment says "will be removed in v2"). It exists because Bubble Tea acquires the terminal before termenv can read OSC responses, so they front-load the query in `init()`.

## How It Was Found

1. Added timing around `NewTerminal()` vs `WaitFor()` — confirmed WaitFor took 5.1s
2. Checked application logs — probe completed in 33ms, binary was idle after
3. Polled vt10x output every 10ms — screen stayed empty for exactly 5.0s then rendered
4. The exact 5s pointed to a timeout constant
5. Found `OSCTimeout = 5 * time.Second` in termenv, called from bubbletea's `init()`
6. Confirmed that `CI=1` makes termenv's `isTTY()` return false, skipping the query

## Fix

Set `CI=1` in the child process environment in `NewTerminal()`. This tells termenv to treat the output as non-TTY, skipping the OSC background-color query entirely. The TUI still renders correctly with alt-screen mode and full styling — the only effect is that termenv assumes a dark background instead of querying for it.

**Before:** each e2e test ~5.8s, suite ~30s
**After:** each e2e test ~0.2-0.8s, suite ~3.3s

## Lesson

When a PTY-spawned TUI program has an unexplained fixed delay, check for terminal capability queries (OSC, DA, DECRQM) that block waiting for responses. Virtual terminal emulators used in testing (vt10x, expect, etc.) typically don't implement these query/response protocols, causing the full timeout to elapse.

The debugging technique that cracked this: polling the virtual terminal at high frequency and noting that the screen was _exactly_ empty for 5.0s. Exact round-number delays almost always point to a timeout constant, not variable work.
