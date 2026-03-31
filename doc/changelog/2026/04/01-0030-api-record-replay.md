# API Record/Replay for E2E Tests

## What

- **Replayer** serves saved API exchanges from JSON fixtures — no network calls
- **Recorder** captures live API traffic via reverse proxy with SSE streaming support
- **APIOption(fixtureDir)** auto-selects replay (default) or record (`E2E_RECORD=1`)
- Fixtures saved to `test/e2e/testdata/<scenario>/exchanges.json`, committed to git
- Interrupted requests (context canceled) are recorded with error info
- Proxy error noise (`http: proxy error: context canceled`) suppressed

## Recording workflow

```sh
# Record (hits real API, requires ANTHROPIC_API_KEY)
E2E_RECORD=1 go test -tags e2e -v -run TestE2E_RecordAPICall -timeout 60s ./test/e2e/

# Replay (no API key, sub-second)
go test -tags e2e -v -run TestE2E_RecordAPICall -timeout 60s ./test/e2e/
```

## Key types

- `Recorder` — HTTP reverse proxy, streams SSE via `recordingBody`, saves JSON
- `Replayer` — serves exchanges in order, returns 502 when exhausted
- `Exchange` — request/response pair with `Error` field for failed requests
- `SaveExchanges()` / `LoadExchanges()` — JSON serialization
