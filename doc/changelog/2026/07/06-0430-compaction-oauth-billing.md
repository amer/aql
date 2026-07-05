# Compaction requests inherit the session's OAuth billing flag

## What changed

`summarizeHistory` now sets `OAuthBilling: a.isOAuth` on the compaction
`ChatParams`, matching what `Run` sends for normal turns.

## Why

Resolves **H6**.

`/compact` and auto-compaction build their own `ChatParams` and left
`OAuthBilling` at its zero value. On an OAuth session every normal turn set the
flag (so the llm adapter injects the billing/beta headers and enables adaptive
thinking), but the compaction call went out mis-configured — a different auth
shape than the rest of the session.

## Design

- One-line fix: the field is read from `a.isOAuth`, the same source `Run` uses.
- `TestCompactHistory_OAuthAgentSetsBilling` injects a capturing `ChatClient`,
  runs `CompactHistory` on an agent built with `WithOAuth()`, and asserts the
  captured params carry `OAuthBilling: true`.

## Note

The finding also flagged the leaky design — every caller re-asserting auth on a
hand-built `ChatParams`. That refactor (a shared params builder) is deferred;
this change fixes the behavior without restructuring the two call sites, whose
params otherwise differ substantially (tools, system prompt, max tokens).
