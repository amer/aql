# Pending ask_user question cleared when the run is torn down

## What changed

`handleEscKey` and `handleStreamError` now set `m.pendingQuestion = nil`.
Previously only `handleAnswerQuestion` cleared it.

## Why

Resolves **C4**.

When the agent calls `ask_user`, the TUI stores the question and its response
channel in `m.pendingQuestion`; the agent's Run goroutine blocks in the
`askUser` callback reading that channel. If the user pressed Esc (cancel) or the
stream errored while a question was pending, the run was torn down — the Run
goroutine returns on `ctx.Done()` and stops reading the channel — but
`pendingQuestion` stayed set.

The next time the user typed a message, `handleSubmit` saw a non-nil
`pendingQuestion` and routed the message into `handleAnswerQuestion`, sending it
down the dead channel instead of submitting it as a new prompt. The message
silently vanished. Clearing `pendingQuestion` on teardown ensures the next
message is treated as a fresh prompt.
