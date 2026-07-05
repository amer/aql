# Bash tool: process-group kill, WaitDelay, and a timeout backstop

## What changed

`execBash` now runs `sh` in its own process group, kills the whole group on
cancellation, sets a `WaitDelay`, and wraps the context with a `bashTimeout`
backstop.

## Why

Resolves **H4**.

`sh -c` can background grandchildren that inherit the command's stdout pipe.
`exec.CommandContext`'s default cancel kills only `sh`, so a backgrounded
grandchild kept the pipe open and `CombinedOutput` blocked until that
grandchild exited — potentially forever. The file's own Q&A claimed "the ctx
already carries a timeout", but nothing enforced one.

## Design

- `SysProcAttr{Setpgid: true}` puts `sh` in a fresh process group; the custom
  `cmd.Cancel` sends `SIGKILL` to the negative PID (the whole group), reaping
  backgrounded grandchildren instead of just `sh`.
- `WaitDelay` (2s) force-closes the I/O pipes if an orphan still lingers, so
  `CombinedOutput` returns instead of blocking on an open write end.
- `context.WithTimeout(ctx, bashTimeout)` (2 min) bounds a command that hangs
  with no output even when no cancel arrives.
- `TestBash_CancelReturnsDespiteOrphanHoldingPipe` drives the exact repro: a
  foreground sleep keeps `sh` alive while a backgrounded sleep holds the pipe;
  the test asserts the tool returns within 5s of cancel.

## Platform note

`Setpgid` and the group kill are POSIX. This matches the project's existing
Unix (darwin/linux) assumptions; a Windows port would isolate these two lines.
