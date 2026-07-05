# Discard partial text when a mid-stream retry re-streams

## What changed

`streamWithRetry` now emits a `StreamReset` event before retrying a failed
attempt that had already streamed text. The stream adapter translates it to an
`AgentStreamResetMsg`, and the TUI clears the in-progress agent text entry so
the retry's fresh stream replaces the abandoned partial instead of appending to
it.

## Why

Resolves **H3**.

On a retryable mid-stream error the runner re-invokes `StreamMessage`, which
re-runs `onText` from the start and re-emits the full response text. The TUI's
`handleStreamDelta` keeps appending deltas to the same `EntryAgentText`, so a
failed first attempt that streamed `"Hello wor"` before erroring left the
transcript showing `"Hello worHello world"` after the retry.

## Design

- `streamWithRetry` wraps `onText` with an `emittedText` flag, reset per
  attempt. The reset is emitted only when the failed attempt actually streamed
  text — a failure before any delta needs no reset, and emitting one could clear
  a prior committed entry.
- Within a single `Run`, multi-iteration turns always have intervening tool
  entries, so the trailing `EntryAgentText` isolates the current attempt's text.
  The handler only clears that trailing entry when it matches the agent.
- `StreamReset` is a new bool field on `domain.StreamEvent`; the adapter follows
  rule 11 (translate, never filter) and the suppression/clearing logic lives in
  the TUI handler.
- Tests at each layer: `TestRunner_StreamResetOnMidStreamRetry` (runner emits
  reset between partial and retry text), `TestForward_StreamReset` (adapter
  translation), `TestModelStreamReset` (handler discards the partial).
