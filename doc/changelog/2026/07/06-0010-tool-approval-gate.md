# Approval gate for side-effecting tools

## What changed

`bash`, `write_file`, `edit`, and `notebook_edit` now require the user to
approve each call before it executes. aql shows a `y/n` prompt with the
command or target path; only an explicit `y`/`yes` runs the tool. A denial is
returned to the model as a tool-error string so it can adapt. Sub-agents
inherit the same gate.

## Why

Resolves **C6**.

Nothing stood between a model's tool call and execution: `bash`/`write_file`/
`edit` ran whatever the model emitted, while `web_fetch` pulled untrusted web
content into the same context — a prompt-injection-to-RCE pipeline. Gating the
side-effecting tools behind explicit consent breaks that chain.

## Design

- `internal/agent/tools/approval.go` — the `ApproverFn` port, the guarded-tool
  set, and a `gate()` wrapper around the executor. With no approver configured
  the executor is unchanged, so existing tests and non-interactive callers are
  unaffected (backward compatible).
- `agent.WithToolOptions` threads tool-executor options (the approver) to every
  spawned child, so sub-agent tool calls are gated too — closing the hole where
  `Spawn()` hand-built child executors.
- `cmd/aql/approval.go` — the interactive prompt, reusing the tested `ask_user`
  prompt path. `approvalPrompt`/`isApproval` are pure and unit-tested.

See `doc/cli/tool-approval.md`.
