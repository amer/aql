# ADR-004: Ports & Adapters for LLM Communication

**Date:** 2026-03-31
**Status:** Accepted

## Context

The agent package (`internal/agent/`) imported `anthropic-sdk-go` types directly
throughout its core logic: `anthropic.MessageParam` in conversation history,
`anthropic.ContentBlockParamUnion` in tool results, `anthropic.Model` for model
selection, and SDK streaming types in the runner loop.

This violated the Ports & Adapters principle from our engineering guidelines:
domain logic should not know about infrastructure. The agent's business rules
(tool loop, history management, compaction) were tangled with SDK-specific
type conversions and API details.

## Decision

Introduce a `domain.ChatClient` interface as the port, and `llm.AnthropicClient`
as the adapter.

### Port (domain layer)

```go
type ChatClient interface {
    StreamMessage(ctx context.Context, params ChatParams, onText func(string)) (*ChatResponse, error)
    SendMessage(ctx context.Context, params ChatParams) (*ChatResponse, error)
}
```

Domain types (`Message`, `ContentBlock`, `ChatParams`, `ChatResponse`) are
provider-agnostic — no SDK imports.

### Adapter (infrastructure layer)

`llm.AnthropicClient` implements `ChatClient`, handling:

- SDK type conversion (domain ↔ SDK)
- Streaming protocol consumption
- OAuth billing header injection
- Authentication (API key vs Bearer token)

### Injection

The agent receives its `ChatClient` via `WithChatClient()` option at construction.
`main.go` constructs the adapter and injects it.

## Consequences

**Benefits:**

- Agent core is testable with fake `ChatClient` implementations (no HTTP servers needed for unit tests)
- Adding a new LLM provider requires only a new adapter, no agent changes
- SDK upgrades are isolated to the adapter
- Clear separation: domain logic vs infrastructure

**Trade-offs:**

- `errors.go` still imports the SDK for `*anthropic.Error` type inspection — acceptable at the infrastructure boundary
- `models/probe.go` still uses the SDK directly for model listing — this is infrastructure, not domain logic
- Slight indirection: `main.go` must construct and inject the adapter

## Alternatives Considered

- **YAGNI / skip it**: We only have one LLM provider. But the principle isn't about
  multi-provider support — it's about separation of concerns and testability.
- **Thin wrapper types only**: Just alias SDK types. Doesn't achieve the goal —
  agent still depends on SDK.
