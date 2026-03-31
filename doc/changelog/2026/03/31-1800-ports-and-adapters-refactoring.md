# Ports & Adapters: Decouple Agent from Anthropic SDK

**Date:** 2026-03-31

## Summary

Introduced a clean Ports & Adapters boundary between the agent's domain logic
and the Anthropic SDK. The agent core no longer imports any SDK types.

## Changes

### New: `internal/domain/types.go`

Provider-agnostic conversation types:

- `Message`, `ContentBlock`, `ToolUseBlock`, `ToolResultBlock` — conversation history
- `ChatClient` interface (the **port**) with `StreamMessage` / `SendMessage`
- `ChatParams`, `ChatResponse`, `ChatToolUse`, `ToolDef` — request/response types

### New: `internal/llm/anthropic.go`

The **adapter** implementing `domain.ChatClient`:

- `AnthropicClient` wraps the Anthropic SDK client
- Handles all SDK type conversions (`toAPIMessages`, `toAPIContentBlocks`, `toAPITools`)
- Manages streaming consumption (`consumeStream`)
- Injects OAuth billing headers when `OAuthBilling` flag is set

### Changed: `internal/agent/`

Agent core is now SDK-free:

- `agent.go` — accepts `domain.ChatClient` via `WithChatClient()` option
- `runner.go` — builds `domain.ChatParams`, calls `chatClient.StreamMessage()`
- `compact.go` — uses `domain.Message` and `chatClient.SendMessage()`
- `tools/defs.go` — removed `ToAPITools()` (moved to adapter)

### Changed: `internal/models/resolve.go`

- `ResolveModel` returns plain `string` instead of `anthropic.Model`
- Model constants defined as plain strings (`ModelSonnet`, `ModelOpus`, `ModelHaiku`)

### Changed: `cmd/aql/main.go`

- Constructs `llm.NewAnthropicClient()` and passes via `agent.WithChatClient()`
- OAuth flag passed separately via `agent.WithOAuth()`

## SDK Import Footprint

Before: ~10+ files imported `anthropic-sdk-go`
After: 4 files (all at infrastructure boundaries):

| File                            | Reason                      |
| ------------------------------- | --------------------------- |
| `internal/llm/anthropic.go`     | Adapter (correct placement) |
| `internal/agent/errors.go`      | SDK error type inspection   |
| `internal/models/probe.go`      | API model listing           |
| `internal/models/model_test.go` | Test only                   |

## Bug Fix

OAuth-derived API keys must be sent via `X-Api-Key` header, not `Authorization: Bearer`.
The initial refactoring incorrectly used Bearer auth, causing 401 errors. Fixed in a
follow-up commit.
