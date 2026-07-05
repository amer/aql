# Roll history back when a turn fails or exhausts tool iterations

## What changed

`Run()` now captures the pre-turn history snapshot and, on either failure exit
— an API error or hitting `maxToolIterations` — emits a `HistoryReplaceMsg`
that restores history to that snapshot via the new `abandonTurn` helper.

## Why

Resolves **H2**.

`Run()` appends the user message to history and emits its `HistoryAppendMsg`
_before_ the API call. If the call then failed, `Run()` returned after emitting
only the error, leaving history ending on that user message. The next turn
appended another user message, so the request carried two consecutive `user`
roles — which the Messages API rejects with a 400 on every subsequent turn
until `/clear`. The tool-iteration-limit exit had the same shape: it ended on a
`user` tool-result message.

## Design

- `preTurnHistory` is the last API-valid state (ends with an assistant message
  or is empty). `snapshotHistory` returns a `len==cap` copy, so appending the
  user message allocates fresh backing and leaves the snapshot untouched.
- `abandonTurn` emits a replace rather than writing `a.history` directly,
  honouring the runner's history-ownership contract (all mutation flows through
  the caller's goroutine via events).
- On the API-error path the replace is emitted before the error event, so even
  a caller that stops draining on error has already received the rollback.
- `TestRunner_FailedTurnRollsBackUserMessage` drives a failed turn followed by a
  successful one and asserts the second request carries no two consecutive user
  roles.

## Trade-off

A failed or exhausted turn is abandoned wholesale: its user message and any
completed tool exchanges are dropped from the API history (they remain visible
in the transcript, which the TUI owns). This keeps history unconditionally
valid; the alternative — trimming only the trailing message — would leave a
dangling `tool_use` with no `tool_result`, which the API also rejects.
