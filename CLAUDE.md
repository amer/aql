## Project

- Go module: `github.com/amer/aql`
- Go version: 1.26.1

## Structure

- `cmd/aql/` — CLI entrypoint
- `internal/` — private packages
- `test/e2e/` — PTY-based e2e test harness (build tag: `e2e`)
- `doc/` — documentation (architecture, changelog, mistakes, adr, api, cli)

## Commands

- `go test ./...` — run all tests
- `go test -v -race -count=1 ./...` — verbose with race detection
- `go build -o bin/aql ./cmd/aql` — build binary
- `go vet ./...` — lint
- `go test -tags e2e -v -count=1 -timeout 60s ./test/e2e/` — run e2e tests (manual, not CI)
- `ANTHROPIC_API_KEY=sk-... go test -tags e2e -v -run TestE2E_RecordAPICall ./test/e2e/` — e2e with API recording

## Rules

- When unsure how to implement a TUI feature, check the Claude Code codebase (github.com/anthropics/claude-code) for inspiration — it's TypeScript, not Go, but the UX patterns and behaviors are the reference implementation
- Always use TDD: write failing tests first, then implement code to make them pass, then refactor
- Follow Functional Core, Imperative Shell: pure functions for logic, thin I/O shell at edges
- Test logic with high-value unit tests on pure functions — avoid brittle tests that break on refactors
- Integration tests only at system boundaries (e.g API calls)
- Commit often: one logical change per commit, after each TDD cycle (test → implement → refactor → commit)
- Do not use Makefiles
- Do not create GitHub Actions workflows
- Use `go` commands directly for build, test, and lint
- Use testify for assertions in tests
- Use `_test` package suffix for external tests (e.g., `package aql_test`)
- Place tests alongside source files (e.g., `aql_test.go` next to `aql.go`)
- Keep `internal/` for private implementation details
- Write errors to stderr, use `os.Exit(1)` for fatal errors
- Use the `run()` pattern in main to allow testable entrypoints

## Documentation

- Document architecture decisions in `doc/adr/`
- Record lessons learned in `doc/mistakes/`
- When introducing or changing CLI commands, document them in `doc/cli/`
- After changing code, update relevant docs to match — code is the source of truth, not docs
- Place changelogs in `doc/changelog/YYYY/MM/` subdirectories (e.g., `doc/changelog/2026/03/`)
- Prefix changelog filenames with day and time: `DD-HHMM-description.md` (e.g., `31-1040-system-prompt-improvements.md`)
- Use descriptive kebab-case for the description portion of changelog filenames

## Logging

- Use structured logging with `log/slog` — never use `fmt.Println` or `log.Printf` for operational logs
- Include good debug-level logs at key decision points, I/O boundaries, and error paths
- Log fields should be meaningful: agent name, event type, duration, error details — not just messages
- Use `slog.Debug` for detailed tracing, `slog.Info` for operational events, `slog.Warn`/`slog.Error` for problems
