# Software Engineering Principles for AI Coding Agents

## Prime Directive

Your job is design engineering, not production. The hard problem is deciding **what** to build and **how** to structure it. Compilation and deployment are trivial. Invest your reasoning in design decisions.

---

## How to Work

### Work in Small Steps

- Make one logical change at a time. Each change should leave the codebase in a working, testable state.
- Prefer many small commits over one large commit. Each step should be reversible.
- Don't batch unrelated changes together. If you're fixing a bug and notice a refactor opportunity, do them as separate steps.

### Test First, Then Implement

- Before writing implementation code, write a failing test that specifies the desired behavior.
- Predict what the test failure message will be. Run the test. Confirm the failure matches your prediction. Then implement.
- Write only enough code to make the test pass. No more.
- After the test passes, refactor if needed while keeping tests green.
- If a test is hard to write, the design is wrong. Fix the design, don't force the test.

### Get Feedback Immediately

- Run tests after every change. Don't accumulate untested changes.
- When something fails, read the actual error. Conjecture boldly, then try to refute with facts.
- Don't guess at what broke. State the problem, form a conjecture, try to refute it.

### Be Empirical, Not Dogmatic

- Don't apply patterns or practices because they sound right. Apply them because they solve the problem in front of you.
- When debugging: start with a conjecture that could be wrong, then try hard to refute it. The "obvious" cause is often wrong.
- Measure before optimizing. Don't guess what's slow - profile it.

---

## How to Design Code

### Separation of Concerns (Most Important Principle)

- **One function, one thing. One class/module, one thing.** If you can describe it with "and," split it.
- **Separate essential complexity from accidental complexity.** Domain logic (business rules, calculations, state transitions) must not know about storage, display, networking, serialization, or infrastructure.
- Domain code should be pure: given inputs, produce outputs. Side effects (I/O, storage, HTTP calls) belong at the edges.
- If you see database queries, HTTP calls, file I/O, or framework-specific code mixed into business logic, extract it.

### Dependency Injection

- Pass dependencies in as parameters. Never create them internally with hardcoded constructors.
- This makes code testable (you can pass fakes), flexible (you can swap implementations), and decoupled (the caller decides what to inject).
- This applies at every scale: function parameters, constructor injection, interface-based design.

### Ports & Adapters

- At every boundary where your code touches external systems (databases, APIs, file systems, third-party libraries), create your own interface (port) and a concrete implementation (adapter).
- Your domain code depends only on the port (your interface). The adapter translates between your interface and the external system.
- This means: your code never imports third-party library types directly into domain logic. Wrap them.
- When the external system changes, only the adapter changes. Domain code is untouched.
- Tests use fake adapters. No real databases, no real HTTP calls in unit tests.

### Modularity

- Break code into small, focused units. Each unit should be understandable without reading the rest of the system.
- Functions: ~10 lines ideal, reject >30 lines. Parameters: reject >5-6.
- If you can't test a unit in isolation, it isn't modular enough. Refactor until you can.
- Modularity is fractal: apply at function, class, package, service, and system levels.

### Cohesion

- Related things go together. Unrelated things go in separate modules.
- Naive cohesion (everything in one file/function) is not cohesion. That's just unstructured code.
- Each module should have access only to what it needs. No God objects, no global state.
- Use domain concepts (bounded contexts, ubiquitous language) to find natural groupings.

### Abstraction

- Hide implementation details. Expose only what consumers need to know.
- Prefer interface types over concrete types in signatures: `Reader` over `FileReader`, `List` over `ArrayList`.
- Don't over-abstract. `interface{}` or `any` everywhere is as bad as no abstraction.
- YAGNI: don't build for hypothetical futures. Build code that is **easy to change** when the future arrives. Abstraction + tests = future insurance.

### Coupling

- Coupling is the root cause of software difficulty. Minimize it relentlessly.
- Prefer loose coupling. The cost of too-tight coupling vastly exceeds the cost of too-loose.
- DRY within a module/package. Between independently deployed services, duplication is cheaper than coupling.
- Async communication between services reduces coupling to network/infrastructure failures.
- If changing code in module A forces changes in module B, the coupling is too tight. Introduce an abstraction.

---

## How to Test

### Test Structure

- Unit tests for pure domain logic. Fast, deterministic, no I/O.
- Use fakes/stubs for external dependencies (databases, APIs, file system). Inject them via dependency injection.
- Integration tests only at system boundaries: real database calls, real HTTP calls. Keep these minimal and focused.
- Acceptance tests as executable specifications of user-visible behavior.

### Test Quality

- Tests must be deterministic. Same code = same result, every run. If a test is flaky, fix the non-determinism (usually concurrency or shared state).
- Tests should specify behavior ("when X happens, expect Y"), not implementation ("method Z was called with args W").
- If tests break on every refactor, they're coupled to implementation, not behavior. Rewrite them.
- Concurrency is the enemy of determinism. Isolate it to controlled edges. Test logic single-threaded.

### Test as Design Feedback

- Hard-to-test code = poorly designed code. The test is telling you something about your design.
- If you need to mock 10 things to test one function, that function has too many dependencies. Refactor.
- If you need a real database to test business logic, your concerns aren't separated. Extract the logic.
- Tests written before code (TDD) produce better abstractions than tests written after.

---

## How to Refactor

- Refactor in tiny steps. Each step preserves behavior (tests stay green).
- Use automated refactoring tools when available (rename, extract method, introduce parameter).
- Refactor toward: shorter functions, fewer parameters, clearer names, separated concerns, injected dependencies.
- Refactor away from: long functions, mixed abstraction levels, hardcoded dependencies, duplicated logic within a module, God classes.
- Don't refactor and change behavior in the same step. One or the other.

---

## Decision Framework

When faced with a design choice, ask:

1. **Is it testable?** Can I write a fast, deterministic test for this in isolation? If not, redesign.
2. **Are concerns separated?** Is domain logic free of infrastructure? If not, extract.
3. **Is it modular?** Can I understand this unit without reading the rest of the system? If not, break it up.
4. **Is coupling appropriate?** Does changing this force changes elsewhere? If so, introduce abstraction.
5. **Is it the simplest thing that works?** Am I adding complexity for hypothetical future needs? If so, remove it.

When debugging:

1. **State the problem.** What did you expect, and how does actual behavior differ?
2. **Form a conjecture** that would explain the discrepancy.
3. **Design an experiment** (a test, a log statement, a reduced reproduction) that would **refute** the conjecture if it's wrong.
4. **Run the experiment.** If the conjecture survives, act on it. If refuted, return to step 2 with a better conjecture.
5. The "obvious" answer is frequently wrong — that's why you try to refute it, not confirm it.

---

## Anti-Patterns to Reject

- Mixing business logic with I/O, storage, or display code
- Functions over 30 lines or with more than 5-6 parameters
- Creating dependencies internally instead of injecting them
- Importing third-party types directly into domain logic
- Writing tests after implementation (leads to implementation-coupled tests)
- Sharing code/libraries between independently deployed services
- Non-deterministic tests (flaky tests that sometimes pass)
- Optimizing before measuring
- Future-proofing with speculative abstractions (YAGNI)
- Big-bang changes instead of small incremental steps
- Skipping the "see the test fail" step in TDD

---

## Code Quality Checklist (Per Change)

- [ ] Tests written first and seen to fail before implementation
- [ ] Each function does one thing, <30 lines, <6 parameters
- [ ] Domain logic is free of I/O, storage, and framework imports
- [ ] External dependencies injected, not hardcoded
- [ ] Third-party APIs wrapped behind own interfaces (Ports & Adapters)
- [ ] Tests are fast, deterministic, and test behavior not implementation
- [ ] Refactoring done as separate step from behavior changes
- [ ] Change is small enough to be easily reversible

---

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

- When unsure how to implement a TUI feature, check the Claude Code codebase (github.com/anthropics/claude-code) for inspiration — it's TypeScript, not Go, but the UX patterns and behaviors are the reference implementation
- Always use TDD: write failing tests first, then implement code to make them pass, then refactor
- Follow Functional Core, Imperative Shell: pure functions for logic, thin I/O shell at edges
- Test logic with high-value unit tests on pure functions — avoid brittle tests that break on refactors
- Integration tests only at system boundaries (API calls, Qdrant, file system)
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
- Document architecture decisions in `doc/adr/`
- Record lessons learned in `doc/mistakes/`
- When introducing or changing CLI commands, document them in `doc/cli/`
- After changing code, update relevant docs to match — code is the source of truth, not docs
- Place changelogs in `doc/changelog/YYYY/MM/` subdirectories (e.g., `doc/changelog/2026/03/`)
- Prefix changelog filenames with day and time: `DD-HHMM-description.md` (e.g., `31-1040-system-prompt-improvements.md`)
- Use descriptive kebab-case for the description portion of changelog filenames
- Use structured logging with `log/slog` — never use `fmt.Println` or `log.Printf` for operational logs
- Include good debug-level logs at key decision points, I/O boundaries, and error paths
- Log fields should be meaningful: agent name, event type, duration, error details — not just messages
- Use `slog.Debug` for detailed tracing, `slog.Info` for operational events, `slog.Warn`/`slog.Error` for problems

## Conventional Commits

All commit messages MUST follow the Conventional Commits specification.

### Format

```text
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

| Type       | Purpose                                                 |
| ---------- | ------------------------------------------------------- |
| `feat`     | A new feature (MINOR in SemVer)                         |
| `fix`      | A bug fix (PATCH in SemVer)                             |
| `docs`     | Documentation only changes                              |
| `style`    | Formatting, whitespace — no code logic changes          |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `perf`     | Performance improvement                                 |
| `test`     | Adding or correcting tests                              |
| `build`    | Build system or dependency changes                      |
| `ci`       | CI configuration changes                                |
| `chore`    | Other changes that don't modify src or test files       |
| `revert`   | Reverts a previous commit                               |

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
