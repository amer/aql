# Late stream deltas after Esc no longer restart the stream

## What changed

`streamState` gains an `interrupted` flag. `handleEscKey` sets it when the user
cancels an in-flight stream; `startStream` clears it. `handleStreamDelta` now
ignores a delta that arrives while `interrupted && !active` — it returns without
restarting the stream.

## Why

Resolves **C3**.

Pressing Esc sets `stream.active = false` and cancels the API context. Deltas
already in the Bubble Tea message queue (enqueued before the cancel landed) then
reached `handleStreamDelta`, whose `!wasStreaming → startStream()` branch flipped
`active` back to true and re-armed the spinner. With no producer left, the
spinner ran forever and input stayed blocked until a second Esc.

The forwarder-side fix (C1/C2) suppresses forwarding once the context is
cancelled, but a delta already handed to the program cannot be recalled, so a
TUI-side guard is still required. The `interrupted` flag distinguishes a stale
post-Esc delta (ignore) from a genuinely new stream's first delta (still
auto-starts, preserving existing behaviour).
