# /compact keeps the transcript until compaction succeeds

## What changed

The `/compact` command no longer clears the on-screen transcript up front. The
chat is now cleared only when compaction actually succeeds, in
`handleCompactDone`.

## Why

Resolves **H8**.

`executeCommand` set `m.chat = nil` the moment `/compact` was entered, then
kicked off the summarization asynchronously. If the compaction API call failed,
`handleCompactDone` appended a "Compact failed" status — but the entire prior
transcript was already gone and could never be recovered. A transient API error
during `/compact` permanently destroyed the user's visible conversation.

## Design

- Removed the premature `m.chat = nil` from the `/compact` branch.
- `handleCompactDone` already nulls `m.chat` on the success path (and estimates
  the new token count from the summary), so success behaves exactly as before.
- On failure the transcript stays intact and the error status is appended below
  it — the user keeps their history and can retry.

## Tests

`TestCompact_FailureAfterCommandPreservesTranscript` drives the full command
flow (`/compact` + enter) and then delivers a failing `CompactDoneMsg`, asserting
the original transcript entry survives. The existing
`TestCompact_DoneReplacesChat` still verifies the success path clears the chat.
