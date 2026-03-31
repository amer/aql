---
paths:
  - "**/*.go"
---

# Refactoring Rules

Refactoring is restructuring code without changing behavior. If you break behavior, you are not refactoring.

## When to Refactor

- **Before** adding a feature — make the change easy, then make the easy change.
- When you struggle to understand code — encode understanding in the code, not comments.
- On the third duplication (Rule of Three).
- Do NOT refactor code you will never modify. Do NOT refactor for aesthetics.
- If rewriting is simpler than refactoring, rewrite — but only behind existing tests.

## When NOT to Refactor

- Code that works, doesn't need changes, and can be treated as a stable API.
- Code with no tests — write tests first, then refactor.
- Do not refactor and change behavior in the same step. One hat at a time.

## Mechanics

1. Ensure tests exist and pass: `go test -race -count=1 ./...`
2. Make one small structural change.
3. Run tests.
4. Run `go vet ./...`.
5. Commit.
6. Repeat.

If a test fails and the cause isn't immediately obvious, `git revert` and redo with smaller steps.

## Key Triggers

- **Extract function:** You pause to figure out what a block does. Name it after intent, not mechanism.
- **Inline function:** The body is as clear as the name, or it's pointless delegation.
- **Extract variable:** A complex expression is hard to read. Name it for its purpose.
- **Extract struct:** A group of fields always travel together.
- **Replace primitive with domain type:** `string` for email, `int` for money — create named types.

## Performance

- Don't let performance concerns block refactoring. Most have negligible impact.
- Finish refactoring first. Benchmark after with `go test -bench` and `pprof`.
- Never guess bottlenecks — `go tool pprof` is the authority.

For full catalog with Go examples, see `doc/go-refactoring-rules.md`.
