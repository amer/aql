# Stream token counting + thinking-block capture and replay

## What changed

`consumeStream` and `SendMessage` in the Anthropic adapter now handle three
things they previously dropped:

- **`message_start` usage** — `input_tokens` is read from `message_start` (the
  documented carrier), not only `message_delta`.
- **Thinking blocks** — thinking text and its signature are captured from the
  stream (`thinking_delta` / `signature_delta`) and from non-streaming
  responses, surfaced on `ChatResponse.Thinking`, and replayed back to the API
  via `toAPIContentBlocks`.
- **Non-streaming tool_use** — `SendMessage` now collects `tool_use` and
  `thinking` blocks, matching `StreamMessage`.

The runner's `buildAssistantMessage` prepends thinking blocks to the assistant
message so they persist in history and replay on the next turn.

New domain types: `ContentBlock.Thinking` (`*ThinkingBlock`),
`ChatResponse.Thinking` (`[]ChatThinking`), and the `ThinkingContentBlock`
constructor.

## Why

Resolves **H10**:

- Input tokens always read 0, so `maybeAutoCompact` (gated on
  `resp.InputTokens > AutoCompactThreshold`) never fired — auto-compaction was
  dead on streaming sessions.
- With OAuth adaptive thinking enabled, the model emits a thinking block before
  `tool_use`. Dropping it meant the follow-up tool-loop request omitted the
  block the API requires (`interleaved-thinking` beta), producing a 400 that
  aborted the turn.
- `SendMessage` silently discarded any `tool_use` the model returned.
