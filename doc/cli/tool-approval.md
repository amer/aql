# Tool Approval Gate

Side-effecting tools require the user's explicit consent before they run. This
prevents a prompt-injected or mistaken model from silently mutating the machine.

## Gated tools

- `bash`
- `write_file`
- `edit`
- `notebook_edit`

Read-only tools (`read_file`, `glob`, `grep`, `list_directory`, `web_search`,
`web_fetch`) run without a prompt.

## Behaviour

When the agent (or a sub-agent) calls a gated tool, aql pauses and shows a
question with the salient argument, e.g.:

```
Allow bash to run: go test ./...? (y/n)
```

- Typing `y` or `yes` (case-insensitive) runs the tool.
- Any other answer denies it. The model receives `tool call to <tool> was
denied by the user` and can adapt.
- Pressing `Esc` cancels the whole turn.

Sub-agents spawned via the `agent` tool inherit the same gate — their
side-effecting calls prompt too.

## Implementation

- The gate itself lives in `internal/agent/tools/approval.go` (`ApproverFn`
  port + `gate` wrapper). With no approver configured, tools run ungated, so
  tests and non-interactive callers are unaffected.
- The interactive prompt is wired in `cmd/aql/approval.go`, reusing the
  `ask_user` prompt path.
- The spawner threads the approver to children via `agent.WithToolOptions`.
