# Fix: sub-agents now inherit OAuth billing and parent options

**Commits:** `25f3f9f`, `0d5671f`

## Problem

Sub-agents spawned via the `agent` tool were constructed with a hand-picked
subset of options (`WithChatClient` + `WithToolExecutor` only). In OAuth mode —
the primary supported auth mode — every child request:

- omitted `OAuthBilling` on `ChatParams` (no billing system-prompt header)
- used `MaxTokens=4096` instead of the OAuth tier's 16384

The bug existed at two layers:

1. `Spawner.Spawn()` dropped the parent's options when creating children.
2. `cmd/aql/main.go` built its own production spawner that bypassed the
   default wiring path, so it needed the fix independently.

## Fix

Propagate the parent's full option slice instead of forwarding individual
fields, so the whole class of bug is fixed — future options inherit
automatically:

- `agent.WithAgentOptions(opts ...Option)` — new `SpawnerOption`; the spawner
  applies these to every child it creates, recursively.
- `Spawn()` builds children with inherited options first, required wiring
  (`WithChatClient`, `WithToolExecutor`) appended last so it always wins.
- `NewToolExecutor` gained a variadic `agentOpts ...Option` parameter (and an
  explicit nil-able `askFn` parameter) and threads them into its spawner.
- `agent.New` passes the agent's own options into the default tool executor.
- `main.go` builds a shared `baseOpts` slice (`WithChatClient`, conditionally
  `WithOAuth`) used by both the primary agent and the production spawner.

## Tests

- `TestAgent_SubAgentsInheritOAuth` — parent spawns child spawns grandchild;
  asserts every captured `ChatParams` has `OAuthBilling=true`.
- `TestSpawner_WithAgentOptions_ChildCarriesOAuthBilling` — asserts
  `OAuthBilling=true` and `MaxTokens=16384` on the child's first call.

## Related docs

- `doc/mistakes/003-duplicated-agent-wiring-drops-config.md`
- `doc/plan/architecture-refactor-plan.md` item 2.1 (marked done)
- `.claude/rules/architecture.md` rule 5 (new spawner rule)
