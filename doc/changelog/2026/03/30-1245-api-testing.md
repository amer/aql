# API Testing Strategy

## Overview

Added offline API testing via SSE fixture replay, with optional live API tests gated
behind an environment variable.

## Fixture Replay (`runner_replay_test.go`)

- Recorded SSE responses stored in `internal/agent/testdata/stream_hello.sse`
- `httptest.NewServer` serves the fixture file with `Content-Type: text/event-stream`
- Agent created with `agent.New(cfg, workDir, agent.WithBaseURL(server.URL))` pointing to the test server
- Runs in normal `go test` — no API key or network required

## Live API Tests (`runner_integration_test.go`)

- Gated by `AQL_LIVE_TEST=1` environment variable
- Requires `ANTHROPIC_API_KEY` to be set
- Used to validate against the real API or re-record fixtures
- Skipped by default in CI and local development

## Base URL Injection

`agent.New(cfg, workDir, agent.WithBaseURL(url))` creates an agent with a custom API
base URL and a dummy API key. This enables injecting test HTTP servers without
touching production code paths.

## SSE Fixture Format

Standard server-sent events format matching the Anthropic streaming API:

```text
event: message_start
data: {"type":"message_start","message":{...}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}

event: message_stop
data: {"type":"message_stop"}
```
