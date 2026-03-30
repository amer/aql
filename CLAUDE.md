# AQL - Agent Rules

## Project

- Go module: `github.com/amer/aql`
- Go version: 1.26.1

## Structure

- `cmd/aql/` — CLI entrypoint
- `internal/` — private packages
- `doc/` — documentation (architecture, changelog, mistakes, adr, api, cli)

## Commands

- `go test ./...` — run all tests
- `go test -v -race -count=1 ./...` — verbose with race detection
- `go build -o bin/aql ./cmd/aql` — build binary
- `go vet ./...` — lint

## Rules

- Always use TDD: write failing tests first, then implement code to make them pass, then refactor
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
- When introducing or changing CLI commands, document them in `doc/cli/`
- After changing code, update relevant docs to match — code is the source of truth, not docs

## Conventional Commits

All commit messages MUST follow the Conventional Commits specification.

### Format

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

| Type       | Purpose                                                   |
|------------|-----------------------------------------------------------|
| `feat`     | A new feature (MINOR in SemVer)                           |
| `fix`      | A bug fix (PATCH in SemVer)                               |
| `docs`     | Documentation only changes                                |
| `style`    | Formatting, whitespace — no code logic changes            |
| `refactor` | Code change that neither fixes a bug nor adds a feature   |
| `perf`     | Performance improvement                                   |
| `test`     | Adding or correcting tests                                |
| `build`    | Build system or dependency changes                        |
| `ci`       | CI configuration changes                                  |
| `chore`    | Other changes that don't modify src or test files         |
| `revert`   | Reverts a previous commit                                 |

### Rules

- Type MUST be lowercase: `feat`, not `Feat`
- Use imperative mood: "add feature" not "added feature"
- No period at end of description
- Description follows colon + space: `feat: add login`
- Limit description to 50-72 characters
- Scope is optional, must be a noun: `feat(auth): add OAuth`
- Body and footers separated by blank lines

### Breaking Changes

- Append `!` after type/scope: `feat!:` or `feat(api)!:`
- Or add `BREAKING CHANGE:` footer
- Breaking changes trigger a MAJOR version bump
