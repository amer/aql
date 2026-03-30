# AQL - Agent Rules

## Project

- Go module: `github.com/amer/aql`
- Go version: 1.26.1

## Structure

- `cmd/aql/` — CLI entrypoint
- `internal/` — private packages
- `doc/` — documentation (architecture, changelog, mistakes, adr, api)

## Commands

- `go test ./...` — run all tests
- `go test -v -race -count=1 ./...` — verbose with race detection
- `go build -o bin/aql ./cmd/aql` — build binary
- `go vet ./...` — lint

## Rules

- Do not use Makefiles
- Do not create GitHub Actions workflows
- Use `go` commands directly for build, test, and lint
- Use testify for assertions in tests
- Use `_test` package suffix for external tests (e.g., `package aql_test`)
- Place tests alongside source files (e.g., `aql_test.go` next to `aql.go`)
- Keep `internal/` for private implementation details
- Write errors to stderr, use `os.Exit(1)` for fatal errors
- Use the `run()` pattern in main to allow testable entrypoints
- Document architecture decisions in `doc/adr/`
- Record lessons learned in `doc/mistakes/`
