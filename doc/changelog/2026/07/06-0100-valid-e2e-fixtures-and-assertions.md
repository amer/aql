# Replace failed-session e2e fixtures with valid ones and assert real behaviour

## What changed

The two replay-backed e2e scenarios now use hand-authored, valid fixtures and
make load-bearing assertions:

- **`record-api-call`** — a proper streaming SSE reply. The test asserts on
  `friend` / "How can I help you today?" from the recorded assistant response
  instead of `WaitFor("hello")`, which the echoed prompt "say hello" satisfied
  on its own.
- **`edit-file-diff`** — drives the full edit round-trip (tool call → C6
  approval prompt → approve → apply → post-edit reply) and asserts the real
  effect: `hello.txt` on disk now contains `goodbye world`. Previously the test
  only `t.Logf`'d and asserted nothing.

## Why

Resolves **C9**.

The committed fixtures were recordings of failed sessions: `record-api-call`
held two `context canceled` exchanges with empty bodies, and `edit-file-diff`
held one 200 plus six 400s and three cancels. The replayer served exchanges by
a single global index, so a background `GET /v1/models` probe racing the
`POST /v1/messages` chat consumed the wrong response — which is how the
recordings ended up corrupt. Combined with vacuous assertions, both tests
passed while exercising nothing.

## Design

- The replayer now keys exchanges by `METHOD PATH` (separate commit) so replay
  is deterministic regardless of arrival order.
- The `/v1/models` fixture returns an empty model list, so the background probe
  finds no candidates and issues zero `POST /v1/messages` probes — leaving the
  chat's POST as the only one, and the multi-turn edit exchanges served in
  recorded order.
- Fixtures carry only `REDACTED` credential headers, matching the new capture-
  time scrubbing (C10).
