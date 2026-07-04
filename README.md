# AQL

AQL is an agent orchestration harness for the terminal, built from scratch in Go. It runs Claude Code-style coding agents: a multi-step agent loop with streaming tool-use, subagent orchestration, session state with automatic context compaction, and a full TUI — no agent framework underneath, just the raw model API.

I built AQL to understand agentic systems from first principles: what it actually takes to run an LLM agent loop reliably — tool execution, state, retries, token/cost accounting, context management — rather than what a framework abstracts away.

## What it does

- **Multi-step agent loop** — model → tool calls → results → model, streamed token-by-token, until the task completes. Retries, error taxonomy, and billing/token accounting are part of the loop, not an afterthought.
- **Tool-use / function calling** — a typed tool registry: file read/write/edit, glob, shell, web fetch, Jupyter notebooks, task tracking, and ask-user escalation.
- **Agent orchestration** — agents can spawn subagents as tools, delegating scoped work with isolated context.
- **Session state & context management** — conversation history, replay, and automatic compaction when the context window fills.
- **Terminal UI** — interactive chat TUI (Bubble Tea/Lipgloss) with markdown rendering, diff views with line numbers, and a model picker.
- **Auth** — Anthropic API keys and OAuth.

## Architecture

```text
cmd/aql            wiring: auth → llm client → agent → tui
internal/agent     the loop: runner, retries, compaction, subagent spawner
internal/agent/tools  tool registry: file, glob, shell, web, notebook, task, ask-user
internal/llm       ports-and-adapters LLM layer (Anthropic adapter)
internal/domain    core types; no dependencies point outward
internal/tui       Bubble Tea app: update/view, streaming render, diffs
internal/stream    token stream handling
test/e2e           end-to-end harness driving the real TUI in a pty (vt10x)
```

Design decisions are recorded as ADRs in [`doc/adr/`](doc/adr/), including the ports-and-adapters LLM boundary and the e2e testing harness.

## Engineering practices

- **TDD throughout** — failing test first; the agent loop's retry, replay, history, streaming, and billing behaviors are each covered by dedicated test suites.
- **E2E tests drive the real terminal** — a pty-based harness renders the actual TUI and asserts on screen state.
- **Conventional Commits**, ADRs, and architecture docs (`doc/architecture/`).

## Status

Early development — APIs and internals change freely. Built and used as a personal daily driver and a learning vehicle for production agentic architecture.

## Getting started

Requires Go 1.26+ and an `ANTHROPIC_API_KEY`.

```sh
go build -o bin/aql ./cmd/aql
./bin/aql
```

Tests and lint:

```sh
go test -race -count=1 ./...
go vet ./...
```

## Contributing

- Follow [Conventional Commits](https://www.conventionalcommits.org/)
- TDD is required — write failing tests first
- Run `git config core.hooksPath .githooks` to enable pre-commit hooks

## License

[MIT](LICENSE)

---

Built by [Amer Jazaerli](https://www.amer.sh) · [linkedin.com/in/amerj](https://www.linkedin.com/in/amerj)
