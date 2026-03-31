# Fix: E2E tests 5s startup delay

**Type:** fix(e2e)

## Summary

Each PTY-based e2e test was blocked for 5 seconds at startup by termenv's OSC
background-color query timing out against the vt10x emulator.

## Changes

- Set `CI=1` in the child process environment in `NewTerminal()` so termenv
  skips the OSC query that vt10x cannot answer.
- Documented root cause and debugging approach in `doc/mistakes/002-e2e-5s-osc-timeout.md`.

## Impact

- E2E suite runtime: ~30s → ~3.3s
- Individual test runtime: ~5.8s → ~0.2-0.8s
- No change to TUI rendering or test fidelity
