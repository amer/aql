---
paths:
  - "**/*.go"
---

# Go Conventions

## Naming

- **Exported**: `PascalCase`. **Unexported**: `camelCase`. No underscores.
- **Short names for short scopes**: `i`, `n`, `r`, `w` in tight loops. Longer scopes need longer names.
- **Interfaces**: `-er` suffix for single-method (`Reader`, `Writer`, `Handler`). Keep interfaces small (1-3 methods).
- **Packages**: short, lowercase, singular nouns (`http`, `auth`, `billing`). Never `util`, `common`, `helpers`.
- **Getters**: `Name()` not `GetName()`. **Setters**: `SetName()`.
- **Acronyms**: all caps — `ID`, `URL`, `HTTP`, `API`.
- **Errors**: variables `ErrNotFound`, types `*NotFoundError`.
- Rename as soon as you find a better name. If you can't name it clearly, fix the design.

## Error Handling

- Errors are values. Handle them explicitly. Never ignore without a comment.
- Wrap with context: `fmt.Errorf("parsing config: %w", err)` — describe what you were doing, not the error.
- Use `errors.Is` and `errors.As` — not string comparison or type assertion.
- Sentinel errors (`var ErrNotFound = errors.New(...)`) for errors callers check. Custom types when callers need data.
- Don't `panic` for expected errors. `panic` is for programmer bugs only.
- Error strings: lowercase, no trailing period.

## Interfaces

- **Define at the consumer, not the producer.** The using package defines what it needs.
- **Accept interfaces, return structs.** Parameters: narrowest interface. Returns: concrete types.
- **Don't create preemptively.** Create when you have a second consumer or need to mock for testing.
- Go interfaces are satisfied implicitly — define them retroactively to decouple.

## Functions and Parameters

- Remove flag arguments (`bool` literals). Replace with two explicit functions or a typed enum.
- If a function can compute a parameter from data it already has, remove the parameter.
- Group parameters that travel together into a struct. Use functional options for optional configuration.
- Command-query separation: functions returning values must not have side effects.

## Guard Clauses

- Use early returns for edge cases, preconditions, error checks. This is idiomatic Go.
- `if err != nil { return ..., err }` — then continue with happy path unindented.
- Never nest the happy path inside `if err == nil`.

## Data Design

- Minimize mutable data scope. Prefer value types. Avoid shared mutable state.
- Value semantics by default. Pointers only for mutation, large structs, or nil semantics.
- No package-level mutable `var` — use dependency injection.
- Design structs so the zero value is useful and valid.

## Structs and Packages

- One struct, one responsibility. If the description uses "and," split.
- Packages represent a single concept: `billing`, `auth`, `storage` — not `models`, `utils`, `types`.
- Unexport aggressively — export only what consumers need.
- Flat is better than deeply nested package trees.

For detailed examples and code smells reference, see `doc/go-refactoring-rules.md`.
