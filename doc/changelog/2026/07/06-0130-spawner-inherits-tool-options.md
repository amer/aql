# Sub-agents inherit ask_user and the shared HTTP client

## What changed

`cmd/aql/main.go` now threads the full tool-executor option set —
`WithApprover`, `WithAskUser`, and `WithHTTPClient` — into the spawner via a
single `sharedToolOpts` slice, instead of passing only `WithApprover`. Added
`TestSpawner_ChildInheritsAskUser` to lock the inheritance contract.

## Why

Resolves **H9**.

The spawner already threads its `toolOpts` to children (via `WithToolOptions`),
but `main.go` only handed it `WithApprover(approve)`. So a sub-agent that
emitted an `ask_user` tool call had no `AskUserFn` and could not reach the user,
and its HTTP calls used a default client rather than the shared one. Children
silently lost configuration the parent had — the exact bug class the spawner's
own file header warns about.

## Design

- The primary executor and the spawner now build from the same
  `[]tools.ExecutorOption` slice, so there is one wiring site and children can
  never drift from the parent.
- `TestSpawner_ChildInheritsAskUser` spawns a child whose scripted client emits
  an `ask_user` tool call and asserts the inherited `AskUserFn` receives the
  question — a regression guard on the contract.
