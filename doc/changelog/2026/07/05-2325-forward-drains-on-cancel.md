# Stream forwarder drains and applies history on cancellation

## What changed

`Forward` and `ForwardWithHistory` no longer return the instant the context is
cancelled. On cancellation they now keep reading the event channel until the
producer closes it:

- `Forward` drains and discards remaining events (`drain`).
- `ForwardWithHistory` drains and still applies every history mutation
  (`drainHistory` / `applyHistory`), but forwards nothing further to the TUI.

## Why

Resolves **C1** and **C2**.

- **C1** — `agent.Run` sends on a buffered (64) channel with unconditional
  `ch <-`. When the forwarder returned on `ctx.Done()` mid-stream, a delta burst
  filled the buffer and the Run goroutine blocked on the send forever, leaking
  the goroutine and the open HTTP stream. Draining to close lets Run finish and
  `defer close(ch)` fire.
- **C2** — cancellation could land between the `HistoryAppendMsg` for the
  assistant `tool_use` (already applied) and the one for its `tool_result`.
  Dropping the tool_result left a dangling `tool_use` in history, which the API
  rejects with a 400 on every subsequent turn until `/clear`. Applying history
  on the drain path keeps the transcript well-formed.

Forwarding is intentionally suppressed after cancel: the TUI resets its own
streaming state on Esc, so a late delta would restart it (that restart is C3,
fixed separately on the TUI side).
