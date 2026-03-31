# E2E Testing Harness

Added a PTY-based end-to-end test harness for testing the `aql` binary as a real user would interact with it.

## What

- `test/e2e/` package with `Terminal` type that spawns the binary in a pseudo-terminal
- Text "screenshots" captured via vt10x terminal emulator — no real display needed
- API call recording via reverse proxy (set `ANTHROPIC_BASE_URL` automatically)
- Timestamped session directories preserve history across runs
- Build-tagged scenarios (`//go:build e2e`) never run in normal `go test ./...`

## Key types

- `Terminal` — PTY + vt10x wrapper with `Send`, `Type`, `SendKey`, `WaitFor`, `Screenshot`, `SaveScreenshot`
- `Recorder` — HTTP reverse proxy that captures request/response exchanges
- `Screenshot` — captured terminal state with `Contains`, `Line`, `Save`

## Artifacts structure

```
test/e2e/artifacts/
    2026-03-31T23-35-15/           # session (timestamped per run)
        api/                        # shared API recordings
        TestE2E_WelcomeScreen/      # per-test screenshots + logs
            001-welcome.txt
            aql.log
```

## Dependencies added

- `github.com/creack/pty/v2` — PTY allocation
- `github.com/hinshun/vt10x` — VT10x terminal emulation

## Running

```sh
# Replay mode (default, fast, no API key needed)
go test -tags e2e -v -count=1 -timeout 60s ./test/e2e/

# Record mode (captures fresh API fixtures)
E2E_RECORD=1 go test -tags e2e -v -run TestE2E_RecordAPICall -timeout 60s ./test/e2e/
```

## ADR

See `doc/adr/005-e2e-testing-harness.md` for design rationale.
