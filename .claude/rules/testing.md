---
paths:
  - "**/*_test.go"
---

# Testing Rules

## TDD Cycle

1. Write a failing test that specifies the desired behavior.
2. Predict the failure message. Run the test. Confirm the failure matches.
3. Write only enough code to make the test pass.
4. Refactor while keeping tests green.
5. If a test is hard to write, the design is wrong. Fix the design, don't force the test.

## Test Structure

- Use `_test` package suffix for external tests (e.g., `package agent_test`).
- Place tests alongside source files (e.g., `agent_test.go` next to `agent.go`).
- Use testify (`assert` + `require`) for assertions, not raw `if` checks.
- Use table-driven tests when there are 3+ cases with the same structure.
- Fresh state per test. Use `t.Cleanup()` for teardown. Never share mutable state between tests.
- Structure: arrange, act, assert.

## Table-Driven Pattern

```go
tests := []struct {
    name    string
    input   string
    want    string
    wantErr bool
}{
    {"empty input", "", "", true},
    {"valid input", "hello", "HELLO", false},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got, err := Transform(tt.input)
        if tt.wantErr {
            require.Error(t, err)
            return
        }
        require.NoError(t, err)
        assert.Equal(t, tt.want, got)
    })
}
```

## Test Quality

- Tests must be deterministic. Same code = same result, every run.
- Test **behavior** ("when X happens, expect Y"), not implementation ("method Z was called").
- If tests break on every refactor, they're coupled to implementation. Rewrite them.
- Use fakes with real behavior, not mocks with expectations.
- Focus on risky and complex areas. Don't chase 100% coverage.
- Probe boundaries: nil slices, empty maps, zero values, negative numbers, context cancellation.
- Bug found? Write the exposing test first, then fix.

## Test Isolation

- Unit tests for pure domain logic. Fast, deterministic, no I/O.
- Use fakes for external dependencies. Inject them via dependency injection.
- Integration tests only at system boundaries (API calls, Qdrant, file system). Keep minimal.
- Tool tests use `t.TempDir()` for filesystem isolation.
- Agent tests use fake `ChatClient` implementations — never hit the real API.
- Run with race detection: `go test -race -count=1 ./...`
