---
paths:
  - "**/*.go"
---

# Concurrency Rules

- **Do not introduce concurrency during refactoring.** Refactoring preserves behavior; adding goroutines changes behavior.
- When refactoring concurrent code, preserve existing synchronization guarantees exactly.
- Extract goroutine bodies into named functions — testable in isolation without concurrency.
- Prefer passing data through channels over sharing memory with mutexes.
- If using shared state, encapsulate the mutex and protected data in the same struct. Never let callers lock/unlock directly.
- `sync.Mutex` fields must not be copied — pass structs containing them by pointer.
- Separate pure logic from concurrent orchestration. Business logic should be testable single-threaded.
- Use the for-select loop pattern for concurrent services that own mutable state.
- Every goroutine must have a shutdown path (accept `context.Context` or a `done` channel).
- Use `go vet` and `-race` flag after every change to concurrent code: `go test -race -count=1 ./...`

For detailed patterns (fan-in, singleflight, errgroup, nil channel toggling, channel API design), see `doc/advanced-go-concurrency-patterns.md`.
