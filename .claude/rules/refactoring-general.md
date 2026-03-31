# Refactoring Rules for AI Coding Agents

## Prime Directive

Refactoring is restructuring code without changing behavior. If you break behavior, you are not refactoring. Every step must preserve all existing tests. If a test fails mid-refactoring and you cannot immediately see why, revert and take smaller steps.

---

## When to Refactor

- Refactor **before** adding a feature — make the change easy, then make the easy change.
- Refactor when you struggle to understand code — encode the understanding in the code itself, not in comments.
- Refactor on the third duplication (Rule of Three) — tolerate it once, wince twice, extract on three.
- Do NOT refactor code you will never modify. Do NOT refactor for aesthetics. Refactor only what you need to change or understand.
- If rewriting is simpler than refactoring, rewrite — but only behind existing tests.

## When NOT to Refactor

- Code that works, doesn't need changes, and can be treated as a stable API.
- Code with no tests — write tests first, then refactor.
- Do not refactor and change behavior in the same step. One hat at a time.

---

## How to Refactor

### Mechanics

1. Ensure tests exist and pass.
2. Make one small structural change.
3. Run tests.
4. Commit.
5. Repeat.

- Each step must leave the codebase in a working state. Never batch multiple refactorings into one untested change.
- If a test fails and the cause isn't immediately obvious, `git revert` and redo with smaller steps.
- Commit after every successful refactoring — these are your checkpoints.

### Performance

- Do NOT let performance concerns block refactoring. Most refactorings have negligible impact.
- Finish refactoring first. Measure after. Optimize only what the profiler identifies.
- Well-factored code is easier to optimize because the hot spot is isolated and obvious.
- Never guess where bottlenecks are — profile.

---

## What to Extract and When

### Extract Function

Trigger: you pause to figure out what a code block does.

- Name the function after **what** it does (intention), not **how** (mechanism).
- If you'd write a comment to explain the block, use that comment as the function name instead.
- Target: ~6 lines per function. Hard reject: >30 lines.
- Hard reject: >5-6 parameters.
- Before extracting, eliminate local variables that complicate scope (Replace Temp with Query, Split Variable).
- Prefer returning a single value. If you need multiple return values, rethink the extraction boundary.

### Inline Function

Trigger: the function body is as clear as the function name, or the function is just pointless delegation.

- Use inlining as an intermediate step: inline two poorly-shaped functions into one blob, then re-extract into better shapes.

### Extract Variable

Trigger: a complex expression is hard to read.

- Name the variable to explain the expression's **purpose**.
- In a class, prefer extracting to a method so other methods can reuse it.

### Inline Variable

Trigger: the variable name adds nothing over the expression itself, or the variable blocks a further refactoring.

---

## Variable and Temp Rules

- Temporary variables encourage long, complex routines. Eliminate them aggressively.
- Replace temps with query methods — this removes parameters from extracted functions and makes logic reusable.
- Remove local variables **before** extracting functions — less scope = easier extraction.
- One variable, one purpose. If a variable is assigned different meanings over its lifetime, split it.
- Declare variables close to first use.
- Prefer immutable bindings (`const`, `:=` with no reassignment). Immutability makes single-responsibility obvious.
- Never reassign input parameters to mean something different.

---

## Naming Rules

- Rename as soon as you find a better name. The cost of a bad name compounds on every future read.
- If you cannot name a function clearly, the design is wrong — fix the design.
- Function names describe **what**, not **how**.
- Getting the name right on the first pass is rare. Revisit names after the code takes shape.

---

## Conditional Logic

### Decompose Conditional

- Extract the condition and each branch into named functions that express **intent**.

### Consolidate Conditional Expression

- When multiple conditions produce the same result, combine into one, then extract with a descriptive name.

### Guard Clauses

- Use early returns for edge cases and preconditions.
- Reserve `if-else` for genuinely equal alternatives.
- Clarity > "single exit point."

### Replace Conditional with Polymorphism

- When the same type-based switch/case appears in multiple places, replace with a class hierarchy.
- Do NOT replace all conditionals with polymorphism — only when it genuinely reduces duplication.

---

## Data Design Rules

- **Mutable data is the #1 source of bugs.** Minimize its scope. Encapsulate it. Prefer immutability.
- All mutable data with scope beyond a single function MUST be behind accessor functions.
- Global mutable data: encapsulate immediately, no exceptions.
- Replace derived variables with query methods — calculations clarify meaning and cannot go stale.
- Never duplicate mutable data. If the same entity exists in multiple places, use a shared reference.
- Fix wrong data structures immediately. Bad data structures corrupt all code that touches them.
- Store money as integer cents. Handle formatting (division by 100) at the display boundary.

### Value Objects vs. References

- Default to immutable value objects (equality by value, no identity).
- Use shared references only when multiple consumers must see the same mutations.

---

## Function and API Design

### Command-Query Separation

- Functions that return values MUST NOT have side effects.
- Functions that cause side effects SHOULD NOT return values.

### Parameter Hygiene

- Remove flag arguments (boolean/enum literals). Replace with two explicit, well-named functions.
- If a function can compute a parameter from data it already has, remove the parameter.
- When extracting multiple fields from an object to pass as separate parameters, pass the whole object instead — unless it creates an unwanted dependency.
- Group parameters that always travel together into a dedicated object or struct.

### Immutability at Boundaries

- If a field should not change after construction, remove its setter. Enforce via constructor.
- To make a function referentially transparent, move internal references to global/mutable state out into parameters.

### Factory Functions over Constructors

- When you need named constructors, flexible return types, or subclass selection logic, use a factory function.

### Command Objects

- Wrap a function in a command object ONLY when you need undo, staged execution, or sub-method decomposition.
- Prefer plain functions 95% of the time.

---

## Class and Module Design

- One class, one responsibility. If the description uses "and," split.
- Test: "If I removed this field, what else becomes nonsensical?" That cluster is a class.
- When a subset of data and methods change together, extract a class.
- Inline a class when it no longer earns its existence.
- Hide delegate objects behind methods to reduce coupling. But when the server becomes a bloated pass-through, remove the middle man.
- Nested functions create hidden data interrelationships. Prefer top-level or module-level placement.

---

## Inheritance vs. Delegation

- Inheritance can only be used once per class. For multiple axes of variation, use delegation.
- Inheritance creates tight coupling — superclass changes can silently break subclasses.
- "Favor composition over inheritance" means **favor a judicious mixture of both**.
- Start with inheritance. Switch to delegation when it becomes a problem.
- Liskov test: if not all superclass methods make sense on the subclass, replace with delegation.
- Pull duplicate subclass methods **up** to the superclass.
- Push methods/fields **down** when only one subclass uses them.
- Extract a superclass when two classes share duplicated data and behavior.
- Collapse a hierarchy when parent and child have converged.

---

## Special Case Pattern

- When many callers check for the same special value and react identically, create a special-case object.
- Special-case objects must be immutable value objects.
- For read-only data, a plain literal record suffices.

---

## Assertions

- Use assertions to make implicit assumptions explicit. They catch bugs at source and communicate intent.
- Assertions are for programmer errors only. External data requires real validation logic.
- Keep assertions even after fixing a bug — they still communicate the invariant.

---

## Code Smells — Detection and Response

| Smell                  | Detection                                          | Action                                                |
| ---------------------- | -------------------------------------------------- | ----------------------------------------------------- |
| Mysterious Name        | You hesitate to explain what it does               | Rename aggressively                                   |
| Duplicated Code        | Same structure in 2+ places                        | Extract Function                                      |
| Long Function          | >30 lines or needs a comment to explain sections   | Decompose into small named functions                  |
| Long Parameter List    | >5-6 parameters                                    | Introduce Parameter Object, Replace Parameter w Query |
| Global Data            | Mutable data accessible from anywhere              | Encapsulate Variable immediately                      |
| Mutable Data           | Widely-scoped mutable state                        | Encapsulate, minimize scope, prefer immutability      |
| Divergent Change       | One module changes for multiple unrelated reasons  | Split Phase, Extract Class                            |
| Shotgun Surgery        | One logical change touches many files              | Move Function, Move Field, consolidate                |
| Feature Envy           | Function uses another module's data more than own  | Move Function to the data                             |
| Data Clumps            | Same group of fields appears together repeatedly   | Extract Class or Introduce Parameter Object           |
| Primitive Obsession    | Primitives for domain concepts (money, dates, etc) | Replace Primitive with Object                         |
| Repeated Switches      | Same type-based switch in multiple places          | Replace Conditional with Polymorphism                 |
| Loops                  | Imperative loops doing filter/map/transform        | Replace Loop with Pipeline                            |
| Lazy Element           | Function/class barely justifying existence         | Inline Function, Inline Class                         |
| Speculative Generality | Hooks for hypothetical future needs                | Remove (YAGNI)                                        |
| Temporary Field        | Field only populated in some code paths            | Extract Class                                         |
| Message Chains         | `a.b().c().d()`                                    | Hide Delegate or extract consuming code               |
| Middle Man             | Class that mostly forwards to another              | Remove Middle Man or Inline                           |
| Insider Trading        | Modules sharing too much internal data             | Move Function, Move Field to fix boundaries           |
| Large Class            | Too many fields, too many methods                  | Extract Class                                         |
| Data Class             | Only fields and getters, no behavior               | Move behavior in                                      |
| Refused Bequest        | Subclass ignores parent's interface                | Replace Subclass/Superclass with Delegate             |
| Comments as deodorant  | Comment explains _what_ code does                  | Extract to well-named function; keep _why_ comments   |

---

## Architecture Rules

- **Design Stamina Hypothesis**: good internal design pays for itself in development speed over time.
- **YAGNI**: build for current needs. Refactoring is cheaper than speculative flexibility.
- **Branch by Abstraction**: for large refactorings, introduce an abstraction layer, migrate gradually, remove the old.
- **Parallel Change (expand-contract)**: for APIs/databases, add the new alongside the old, migrate consumers, remove the old.
- Long-lived branches make refactoring dangerous. Integrate continuously.
- **Camping rule**: always leave the codebase healthier than you found it.

---

## Testing Rules for Refactoring

- Before refactoring, ensure self-checking tests exist. If they don't, write them first.
- TDD cycle: write a failing test, implement minimum code to pass, refactor while green.
- Fresh fixture per test. Never share mutable state between tests.
- Structure: setup, exercise, verify.
- Test **behavior**, not implementation. Tests coupled to implementation break on every refactor.
- Focus on risky and complex areas. Don't chase 100% coverage.
- Probe boundaries: empty collections, zeros, negatives, blanks, invalid inputs.
- Bug found? Write the exposing test first, then fix.
- Hard-to-test code = bad design. The test is feedback, not the problem.

---

## Decision Framework

When making a structural decision, ask in order:

1. **Is it testable?** Can I write a fast, deterministic test in isolation? If not, redesign.
2. **Are concerns separated?** Is domain logic free of infrastructure? If not, extract.
3. **Is it modular?** Can I understand this unit without reading the rest of the system? If not, decompose.
4. **Is coupling minimal?** Does changing this force changes elsewhere? If so, introduce an abstraction.
5. **Is it the simplest thing that works?** Am I adding complexity for hypothetical futures? If so, remove it.

When debugging during refactoring:

1. State the problem: expected vs. actual.
2. Form a conjecture.
3. Design an experiment that would **refute** the conjecture.
4. Run it. If refuted, form a better conjecture. If confirmed, act on it.
