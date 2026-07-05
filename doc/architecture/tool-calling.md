# Tool Calling

How AQL advertises tools to the model, decodes tool calls from the stream,
dispatches them, and feeds results back.

## Key insight

The model does **not** embed magic codes in its text. The Anthropic API
returns structured `tool_use` content blocks alongside text — typed JSON
blocks with an ID, a tool name, and JSON input. Dispatch is a map lookup by
name; results go back as `tool_result` blocks matched by ID.

## The mechanism, conceptually

The model only ever generates tokens. There is no "call" instruction in the
network — tool calling is one unified convention layered on token generation:

1. **Schemas are context text.** The tool definitions sent with each request
   are rendered into the model's context. The model reads them like any
   other text.
2. **The decision is learned, not mechanical.** The model was trained so
   that emitting a tool call is the highest-probability continuation when a
   tool would help. Choosing to call `read_file` is the same act as choosing
   the next word.
3. **One meta-tool, one degree of freedom.** There are not N call mechanisms
   for N tools. The model has a single trained output format: a delimited
   block containing `{name, input JSON}`. Every tool is just a different
   `name` value and JSON shape. Adding a tool adds zero mechanism — only new
   context text and one more valid `name` string.
4. **The API server is the parser.** It lifts the delimited block out of the
   token stream and returns it as a typed `tool_use` content block with
   `stop_reason: tool_use`. Generation halts — the model cannot execute
   anything; it stops and waits.
5. **The harness closes the loop.** It runs the tool and sends the output
   back as a `tool_result` block; the model resumes with the result in
   context.

## Multiple tool calls: matching, blocking, replay

- **Matching is by ID, not order.** Each `tool_use` block carries a unique
  ID (`toolu_...`). Each `tool_result` sent back carries the ID of the call
  it answers. The model pairs them by reading both in context.
- **One linear context.** Parallel calls sit in one assistant message; all
  results return together in one user-role message. There are no per-tool
  channels.
- **The turn is a barrier.** The next API call must include a result for
  every tool call the model made — the API rejects partial results. AQL runs
  tools concurrently (one goroutine each, `runToolsParallel`) but waits for
  all before resuming. The slowest tool gates the turn. This blocks only
  that agent: a sub-agent is a separate conversation running its own loop,
  appearing to the parent as one slow tool call.
- **Stateless full replay.** The API keeps no session. Every round-trip
  resends the entire history: the assistant message (text + `tool_use`
  blocks, replayed verbatim so the IDs are present) plus a new `Role: user`
  message containing the `tool_result` blocks. No human typed anything —
  "user" is just the role that carries results. Consequences: cost grows
  with the transcript (hence prompt caching and AQL's auto-compaction), and
  results cannot be sent without replaying the matching `tool_use` message.

## Sequence

```mermaid
sequenceDiagram
    participant U as User (TUI)
    participant R as Agent.Run()<br/>runner.go
    participant C as ChatClient<br/>llm/anthropic.go
    participant API as Claude API
    participant X as ExecutorFn<br/>tools/defs.go
    participant H as toolHandler<br/>(registry map)

    U->>R: Run(ctx, "fix the bug")
    R->>R: snapshot history, append user msg

    loop until end_turn (max 25 iterations)
        R->>R: buildChatParamsFrom()<br/>attach tool schemas (Definitions())
        R->>C: StreamMessage(params, onText)
        C->>API: POST /v1/messages (stream)

        API-->>C: SSE: text deltas
        C-->>R: onText(delta)
        R-->>U: StreamEvent{Text}

        API-->>C: SSE: content_block_start (tool_use, ID, name)
        API-->>C: SSE: input_json_delta (partial JSON...)
        C->>C: accumulate into pendingToolUse
        C-->>R: ChatResponse{ToolUses, StopReason}

        alt no tool uses / end_turn
            R-->>U: StreamEvent{Done}
        else has tool uses
            R-->>U: StreamEvent{ToolCall} (per tool)
            par one goroutine per tool
                R->>X: toolExecutor(ctx, workDir, name, input)
                X->>H: registry[name] lookup
                H-->>X: (output string, err)
                X-->>R: output
            end
            R-->>U: StreamEvent{ToolDone} (per tool)
            R->>R: wrap outputs as tool_result blocks<br/>(matched by tool_use ID)
            R->>R: append as Role:user message to history
            Note over R,API: loop back — next API call<br/>includes the tool results
        end
    end
```

## The four stages, with source locations

### 1. Advertising tools to the model

`buildChatParamsFrom()` (`internal/agent/runner.go`) attaches
`tools.Definitions()` — name, description, and JSON input schema per tool —
to every API request. This is how the model knows what it can call and what
arguments each tool takes.

### 2. Decoding tool calls from the stream

`consumeStream()` (`internal/llm/anthropic.go`) reads typed SSE events:

- `content_block_start` with `type: "tool_use"` opens a tool call, carrying
  its ID and name.
- `input_json_delta` events deliver the input JSON in chunks; they are
  accumulated into `pendingToolUse.inputBuf`.
- Text deltas stream to the TUI via `onText`; completed tool calls collect
  into `ChatResponse.ToolUses`.

### 3. Dispatch — the lookup table

`buildRegistry()` (`internal/agent/tools/defs.go`) is the
name → handler map:

```go
return map[string]toolHandler{
    "read_file": withDir(execReadFile),
    "bash":      execBash,
    ...
}
```

Every handler has the uniform signature
`func(ctx, workDir string, input json.RawMessage) (string, error)`.
Stateful tools (`task_*`, `agent`) are added by `register*Tools()` inside
`NewExecutor()`. `execute()` does a plain map lookup by the name the model
sent; unknown names return a Go error (infrastructure failure). Tool-level
failures are returned as the string value — see architecture rule 4.

### 4. Feeding results back

The tool loop in `Run()` (`internal/agent/runner.go`):

1. If `resp.ToolUses` is non-empty, `executeTools()` runs all calls in
   parallel goroutines (`runToolsParallel`).
2. `emitToolResults()` wraps each output as a `tool_result` content block
   keyed by the originating `tool_use` ID:
   `domain.ToolResultContentBlock(tu.ID, r.output, r.isError)`.
3. The blocks become a new message with `Role: user` — the API convention
   for returning tool results — appended to local history.
4. The loop calls the API again. The model reads the results and either
   calls more tools or answers with text. `end_turn` or zero tool uses
   exits; `maxToolIterations` (25) is the safety cap.

## Related

- `doc/architecture/overview.md` — package structure and event flow
- `.claude/rules/architecture.md` rules 4 (tool error convention) and 9
  (three-part tool registration)
