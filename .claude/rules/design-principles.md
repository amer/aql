# Software Engineering Principles

## Prime Directive

Your job is design engineering, not production. The hard problem is deciding **what** to build and **how** to structure it. Invest your reasoning in design decisions.

## How to Work

- Make one logical change at a time. Each change should leave the codebase in a working, testable state.
- Prefer many small commits over one large commit. Each step should be reversible.
- Don't batch unrelated changes together.
- Before writing implementation code, write a failing test that specifies the desired behavior.
- Predict the failure message. Run the test. Confirm. Then implement.
- Write only enough code to make the test pass. No more.
- Run tests after every change. Don't accumulate untested changes.
- Don't apply patterns because they sound right. Apply them because they solve the problem in front of you.
- When debugging: conjecture boldly, then try to refute with facts. The "obvious" cause is often wrong.
- Measure before optimizing. Don't guess what's slow — profile it.

## How to Design Code

- **Separation of Concerns:** One function, one thing. One module, one thing. If you can describe it with "and," split it.
- **Pure domain logic:** Domain code should be pure: given inputs, produce outputs. Side effects belong at the edges.
- **Dependency Injection:** Pass dependencies in as parameters. Never create them internally with hardcoded constructors.
- **Ports & Adapters:** At every boundary with external systems, create your own interface (port) and a concrete implementation (adapter). Domain code depends only on ports.
- **Modularity:** Functions ~10 lines ideal, reject >30 lines. Parameters: reject >5-6. If you can't test a unit in isolation, it isn't modular enough.
- **Cohesion:** Related things go together. No God objects, no global state. Use domain concepts for natural groupings.
- **Abstraction:** Hide implementation details. Prefer interface types over concrete types in signatures. Don't over-abstract.
- **Coupling:** Minimize relentlessly. DRY within a module. Between services, duplication is cheaper than coupling. If changing module A forces changes in module B, introduce an abstraction.
- **YAGNI:** Don't build for hypothetical futures. Build code that is easy to change when the future arrives.

## Decision Framework

When faced with a design choice:

1. **Is it testable?** Can I write a fast, deterministic test in isolation? If not, redesign.
2. **Are concerns separated?** Is domain logic free of infrastructure? If not, extract.
3. **Is it modular?** Can I understand this unit without reading the rest of the system? If not, break it up.
4. **Is coupling appropriate?** Does changing this force changes elsewhere? If so, introduce abstraction.
5. **Is it the simplest thing that works?** Am I adding complexity for hypothetical futures? If so, remove it.

When debugging:

1. State the problem: expected vs. actual.
2. Form a conjecture that would explain the discrepancy.
3. Design an experiment that would **refute** the conjecture if it's wrong.
4. Run it. If refuted, form a better conjecture. If confirmed, act on it.

## Anti-Patterns to Reject

- Mixing business logic with I/O, storage, or display code
- Functions over 30 lines or with more than 5-6 parameters
- Creating dependencies internally instead of injecting them
- Importing third-party types directly into domain logic
- Writing tests after implementation (leads to implementation-coupled tests)
- Non-deterministic tests (flaky tests that sometimes pass)
- Optimizing before measuring
- Future-proofing with speculative abstractions
- Big-bang changes instead of small incremental steps
- Skipping the "see the test fail" step in TDD
