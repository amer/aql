# Claude API Streaming

## SDK

Official Go SDK: `github.com/anthropics/anthropic-sdk-go`

The client reads `ANTHROPIC_API_KEY` from the environment automatically.

## Streaming Usage

```go
import "github.com/anthropics/anthropic-sdk-go"

client := anthropic.NewClient()

stream := client.Messages.NewStreaming(context.TODO(), anthropic.MessageNewParams{
    Model:     anthropic.ModelClaudeOpus4_6,
    MaxTokens: 1024,
    Messages: []anthropic.MessageParam{
        anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
    },
})

for stream.Next() {
    event := stream.Current()
    switch eventVariant := event.AsAny().(type) {
    case anthropic.ContentBlockDeltaEvent:
        switch deltaVariant := eventVariant.Delta.AsAny().(type) {
        case anthropic.TextDelta:
            fmt.Print(deltaVariant.Text)
        }
    }
}
if err := stream.Err(); err != nil {
    log.Fatal(err)
}
```

## SSE Event Types

The stream emits server-sent events (SSE). Key event types:

| Event                 | Description                               |
| --------------------- | ----------------------------------------- |
| `message_start`       | Contains the initial `Message` object     |
| `content_block_start` | Start of a content block (text, tool use) |
| `content_block_delta` | Incremental update to a content block     |
| `content_block_stop`  | End of a content block                    |
| `message_delta`       | Top-level changes (stop reason, usage)    |
| `message_stop`        | End of the message                        |
| `ping`                | Keep-alive event                          |
| `error`               | Error during streaming                    |

## Non-Streaming Alternative

For cases where you don't need real-time text output, accumulate the full message:

```go
stream := client.Messages.NewStreaming(context.TODO(), anthropic.MessageNewParams{
    Model:     anthropic.ModelClaudeOpus4_6,
    MaxTokens: 128000,
    Messages: []anthropic.MessageParam{
        anthropic.NewUserMessage(anthropic.NewTextBlock("Write a detailed analysis...")),
    },
})

message := anthropic.Message{}
for stream.Next() {
    event := stream.Current()
    message.Accumulate(event)
}
if err := stream.Err(); err != nil {
    log.Fatal(err)
}
fmt.Println(message.Content[0].Text)
```

## Reference

- [API docs](https://docs.anthropic.com/en/api/messages-streaming)
- [Go SDK](https://github.com/anthropics/anthropic-sdk-go)
