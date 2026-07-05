# Fix data race on the stream cancel func

## What changed

Replaced the shared `*context.CancelFunc` threaded through `configureTUI` with a
mutex-guarded `streamCanceller` type (`cmd/aql/stream_cancel.go`). `onSubmit`
now calls `streamCancel.set(cancel)` and the TUI's cancel callback is
`streamCancel.cancelActive`.

## Why

Resolves **C5**. The cancel func was written from the `tea.Cmd` goroutine that
starts a stream and read from the Update-loop goroutine when the user pressed
Esc, with no synchronization — a data race the `-race` detector flags. Bubble
Tea runs Cmds in their own goroutines, so the two accesses genuinely overlap.
Encapsulating the field with its mutex removes the race and the nil-check
boilerplate at the call site.
