# 003 — Duplicated agent wiring silently drops configuration

**Date:** 2026-07-05
**Fixed by:** `25f3f9f`, `0d5671f`

## What happened

Sub-agents spawned by the `agent` tool ran without OAuth billing and with the
wrong max-tokens limit. No error surfaced — children just made silently
degraded API calls (missing billing header, 4096 instead of 16384 tokens).

## Root cause

Agent construction knowledge was duplicated in two places, and both copies
drifted from the real wiring:

1. `Spawner.Spawn()` hand-picked the options for child agents
   (`WithChatClient` + `WithToolExecutor`), re-implementing what `main.go`
   knew. It never learned about `WithOAuth()`, so children lost it.
2. `main.go` built its own production `Spawner` directly instead of going
   through the default `NewToolExecutor` path — so fixing `Spawn()` alone was
   not enough; the production spawner also had to receive the base options.

The failure mode is structural: every hand-picked forwarding site must be
updated whenever a new option is added, and nothing fails when it isn't.

## Lesson

- Never construct a child/derived component from a hand-picked subset of the
  parent's options. Propagate the whole option set and append required wiring
  last so it wins. Implemented as `agent.WithAgentOptions`.
- When fixing duplicated construction, grep for every construction site —
  the production wiring (`main.go`) had a second, independent copy of the bug.
- A config-dropping bug produces no error. Only a test that captures the
  outgoing `ChatParams` from a spawned child could catch it; assert on what
  crosses the boundary, not on internal fields.

## Guardrails added

- `.claude/rules/architecture.md` rule 5: spawners must receive parent base
  options via `WithAgentOptions`; never hand-pick child options.
- `internal/agent/spawner.go` file-guideline Q&A documents the inheritance
  path.
- Regression tests: `TestAgent_SubAgentsInheritOAuth` (recursive, depth 2),
  `TestSpawner_WithAgentOptions_ChildCarriesOAuthBilling`.
