# AQL

AQL is a terminal-based tool for agentic coding. It provides an interactive TUI for working with AI coding agents directly from your terminal.

## Status

Early development — not yet ready for use.

## Getting Started

### Prerequisites

- Go 1.26.1+

### Environment

```sh
source scripts/env.sh
```

This loads the `ANTHROPIC_API_KEY` from `secrets/`. The `secrets/` directory is gitignored.

### Build

```sh
go build -o bin/aql ./cmd/aql
```

### Run

```sh
./bin/aql
```

### Test

```sh
go test ./...
go test -v -race -count=1 ./...  # verbose with race detection
```

### Lint

```sh
go vet ./...
```

## Project Structure

```text
cmd/aql/           CLI entrypoint
internal/           Private packages
scripts/            Utility scripts
doc/
  architecture/     System design and component docs
  changelog/        Release notes
  mistakes/         Lessons learned and post-mortems
  adr/              Architecture decision records
  api/              API reference
```

## Contributing

- Follow [Conventional Commits](https://www.conventionalcommits.org/) for all commit messages
- TDD is required — write failing tests first
- Run `git config core.hooksPath .githooks` to enable pre-commit hooks

## License

TBD
