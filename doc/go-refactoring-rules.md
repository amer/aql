# Refactoring Rules for AI Coding Agents — Go Edition

Adapted from general refactoring principles to idiomatic Go. Every rule uses Go constructs, tooling, and conventions.

---

## Prime Directive

Refactoring is restructuring code without changing behavior. If you break behavior, you are not refactoring. Every step must preserve all existing tests (`go test ./...` stays green). If a test fails mid-refactoring and you cannot immediately see why, `git revert` and take smaller steps.

---

## When to Refactor

- Refactor **before** adding a feature — make the change easy, then make the easy change.
- Refactor when you struggle to understand code — encode the understanding in the code itself, not in comments.
- Refactor on the third duplication (Rule of Three) — tolerate it once, wince twice, extract on three.
- Do NOT refactor code you will never modify. Do NOT refactor for aesthetics. Refactor only what you need to change or understand.
- If rewriting is simpler than refactoring, rewrite — but only behind existing tests.

## When NOT to Refactor

- Code that works, doesn't need changes, and can be treated as a stable API.
- Code with no tests — write tests first (`_test.go`), then refactor.
- Do not refactor and change behavior in the same step. One hat at a time.

---

## How to Refactor

### Mechanics

1. Ensure tests exist and pass: `go test -race -count=1 ./...`
2. Make one small structural change.
3. Run tests.
4. Run `go vet ./...` to catch subtle issues.
5. Commit.
6. Repeat.

Each step must leave the codebase in a working state. Never batch multiple refactorings into one untested change. If a test fails and the cause isn't immediately obvious, `git revert` and redo with smaller steps.

### Performance

- Do NOT let performance concerns block refactoring. Most refactorings have negligible impact.
- Finish refactoring first. Measure after with `go test -bench` and `pprof`.
- Well-factored code is easier to optimize because the hot spot is isolated and obvious.
- Never guess where bottlenecks are — `go tool pprof` is your authority.

---

## Go-Specific Refactoring Tooling

```bash
# Rename across the entire module (type, function, variable, field)
gorename -from '"github.com/amer/aql".OldName' -to NewName

# Move packages or reorganize structure
gomvpkg -from github.com/amer/aql/old/path -to github.com/amer/aql/new/path

# Find all callers / references
guru callers ./internal/agent.Run
guru referrers ./internal/llm.Client

# Detect dead code
staticcheck -checks U1000 ./...

# Race detection during refactoring
go test -race -count=1 ./...

# Benchmark before and after
go test -bench=BenchmarkX -benchmem -count=5 ./... | tee before.txt
# ... refactor ...
go test -bench=BenchmarkX -benchmem -count=5 ./... | tee after.txt
benchstat before.txt after.txt
```

---

## What to Extract and When

### Extract Function

Trigger: you pause to figure out what a code block does.

```go
// BEFORE: inline logic
func processOrder(order Order) error {
    // validate
    if order.Total <= 0 {
        return fmt.Errorf("invalid total: %d", order.Total)
    }
    if order.CustomerID == "" {
        return fmt.Errorf("missing customer ID")
    }
    // ... 20 more lines of processing
}

// AFTER: extracted with intention-revealing name
func processOrder(order Order) error {
    if err := validateOrder(order); err != nil {
        return err
    }
    // ... processing is now focused
}

func validateOrder(order Order) error {
    if order.Total <= 0 {
        return fmt.Errorf("invalid total: %d", order.Total)
    }
    if order.CustomerID == "" {
        return fmt.Errorf("missing customer ID")
    }
    return nil
}
```

- Name the function after **what** it does (intention), not **how** (mechanism).
- If you'd write a comment to explain the block, use that comment as the function name instead.
- Target: ~10 lines per function. Hard reject: >30 lines.
- Hard reject: >5-6 parameters.
- In Go, multiple return values are idiomatic — but if you're returning more than `(T, error)`, consider a result struct.

### Inline Function

Trigger: the function body is as clear as the function name, or the function is just pointless delegation.

```go
// BEFORE: pointless delegation
func isActive(u User) bool {
    return u.Status == StatusActive
}

// AFTER: inline at call site — the expression is already clear
if u.Status == StatusActive { ... }
```

Use inlining as an intermediate step: inline two poorly-shaped functions into one blob, then re-extract into better shapes.

### Extract Variable

Trigger: a complex expression is hard to read.

```go
// BEFORE
if req.Header.Get("Authorization") != "" && time.Since(session.CreatedAt) < 24*time.Hour {

// AFTER
hasAuth := req.Header.Get("Authorization") != ""
sessionValid := time.Since(session.CreatedAt) < 24*time.Hour
if hasAuth && sessionValid {
```

### Inline Variable

Trigger: the variable name adds nothing over the expression itself.

```go
// BEFORE: variable adds no clarity
baseURL := "https://api.example.com"
resp, err := http.Get(baseURL)

// AFTER
resp, err := http.Get("https://api.example.com")
```

---

## Variable and Temp Rules

- Temporary variables encourage long, complex routines. Eliminate them aggressively.
- One variable, one purpose. If a variable is assigned different meanings over its lifetime, split it.
- Declare variables close to first use — Go's `:=` encourages this naturally.
- Prefer `:=` for local scope. Use `var` only when zero-value initialization is intentional and meaningful.
- Never reassign input parameters to mean something different.
- Avoid naked `var` blocks at the top of functions — declare where used.

```go
// BEFORE: variable reused for different purposes
result := fetchFromCache(key)
if result == nil {
    result = fetchFromDB(key) // same var, different meaning
}

// AFTER: distinct names, distinct purposes
cached := fetchFromCache(key)
if cached != nil {
    return cached, nil
}
fresh := fetchFromDB(key)
```

---

## Naming Rules — Go Conventions

- Rename as soon as you find a better name. The cost of a bad name compounds on every future read.
- If you cannot name a function clearly, the design is wrong — fix the design.
- Function names describe **what**, not **how**.

### Go-Specific Naming

| Convention                                      | Example                            | Anti-Pattern                    |
| ----------------------------------------------- | ---------------------------------- | ------------------------------- |
| MixedCaps, no underscores                       | `ReadAll`, `parseToken`            | `read_all`, `Parse_Token`       |
| Short receiver names (1-2 chars)                | `func (s *Server) Serve()`         | `func (server *Server) Serve()` |
| Acronyms are all-caps                           | `HTTPClient`, `userID`             | `HttpClient`, `userId`          |
| Getters have no `Get` prefix                    | `func (u *User) Name() string`     | `func (u *User) GetName()`      |
| Setters use `Set` prefix                        | `func (u *User) SetName(n string)` |                                 |
| Interface names: `-er` suffix for single-method | `Reader`, `Writer`, `Closer`       | `IReader`, `ReaderInterface`    |
| Package names: short, lowercase, no plural      | `http`, `json`, `auth`             | `httpUtils`, `jsonHelpers`      |
| Exported = public, unexported = private         | `ParseToken` vs `parseToken`       | Using `_` prefix for private    |
| Error variables: `Err` prefix                   | `ErrNotFound`                      | `NotFoundError`                 |
| Error types: `Error` suffix                     | `type PathError struct`            | `type ErrPath struct`           |

```go
// Go proverb: "A little copying is better than a little dependency."
// Within a module, extract. Across module boundaries, sometimes duplicate.
```

---

## Error Handling Refactoring

### Wrap Errors with Context

```go
// BEFORE: bare error return loses context
func loadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    // ...
}

// AFTER: wrapped with context
func loadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("load config %s: %w", path, err)
    }
    // ...
}
```

### Extract Error Sentinel Values

```go
// BEFORE: string comparison
if err.Error() == "not found" { ... }

// AFTER: sentinel error
var ErrNotFound = errors.New("not found")

if errors.Is(err, ErrNotFound) { ... }
```

### Replace Type Assertion Chains with errors.As

```go
// BEFORE
if e, ok := err.(*PathError); ok { ... }

// AFTER — works through wrapped errors
var pathErr *PathError
if errors.As(err, &pathErr) { ... }
```

---

## Interface Refactoring — Go Idioms

### Accept Interfaces, Return Structs

```go
// BEFORE: concrete dependency
func NewService(db *PostgresDB) *Service { ... }

// AFTER: interface dependency — testable, decoupled
type Store interface {
    Get(ctx context.Context, id string) (*Item, error)
    Put(ctx context.Context, item *Item) error
}

func NewService(store Store) *Service { ... }
```

### Keep Interfaces Small

```go
// BEFORE: fat interface
type Repository interface {
    Get(id string) (*User, error)
    List() ([]*User, error)
    Create(u *User) error
    Update(u *User) error
    Delete(id string) error
    Search(query string) ([]*User, error)
    Count() (int, error)
}

// AFTER: segregated by use case
type UserReader interface {
    Get(ctx context.Context, id string) (*User, error)
}

type UserWriter interface {
    Create(ctx context.Context, u *User) error
    Update(ctx context.Context, u *User) error
}

// Compose when needed
type UserStore interface {
    UserReader
    UserWriter
}
```

### Define Interfaces at the Consumer, Not the Producer

```go
// WRONG: interface defined next to implementation
// internal/database/store.go
type Store interface { ... }
type PostgresStore struct { ... }

// RIGHT: interface defined where it's needed
// internal/agent/agent.go
type MessageStore interface {
    Save(ctx context.Context, msg Message) error
}

// internal/database/store.go — just the concrete type
type PostgresStore struct { ... }
func (s *PostgresStore) Save(ctx context.Context, msg Message) error { ... }
```

Go interfaces are satisfied implicitly — the producer never needs to know about the consumer's interface.

### Extract Interface from Usage

When you find a function depending on a concrete type but only using 2-3 methods:

```go
// BEFORE
func process(client *http.Client) { ... } // only calls client.Do()

// AFTER
type HTTPDoer interface {
    Do(req *http.Request) (*http.Response, error)
}

func process(client HTTPDoer) { ... }
```

---

## Struct Refactoring

### Extract Struct

When a group of fields always travel together:

```go
// BEFORE
type Order struct {
    ID          string
    Total       int
    BillingName string
    BillingAddr string
    BillingCity string
    BillingZip  string
    ShipName    string
    ShipAddr    string
    ShipCity    string
    ShipZip     string
}

// AFTER
type Address struct {
    Name string
    Addr string
    City string
    Zip  string
}

type Order struct {
    ID      string
    Total   int
    Billing Address
    Shipping Address
}
```

### Replace Primitive with Domain Type

```go
// BEFORE: primitive obsession
func Charge(amountCents int, currency string) error { ... }

// AFTER: domain type enforces invariants
type Money struct {
    Cents    int64
    Currency string
}

func Charge(amount Money) error { ... }
```

### Functional Options for Complex Construction

When a constructor accumulates too many parameters:

```go
// BEFORE: parameter explosion
func NewServer(addr string, port int, timeout time.Duration, maxConns int, tls bool) *Server

// AFTER: functional options
type Option func(*Server)

func WithTimeout(d time.Duration) Option {
    return func(s *Server) { s.timeout = d }
}

func WithMaxConns(n int) Option {
    return func(s *Server) { s.maxConns = n }
}

func NewServer(addr string, opts ...Option) *Server {
    s := &Server{addr: addr, timeout: 30 * time.Second, maxConns: 100}
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

---

## Conditional Logic — Go Idioms

### Guard Clauses (Early Returns)

```go
// BEFORE: nested conditionals
func process(order *Order) error {
    if order != nil {
        if order.IsValid() {
            if order.Total > 0 {
                // actual logic buried 3 levels deep
                return charge(order)
            } else {
                return errors.New("zero total")
            }
        } else {
            return errors.New("invalid order")
        }
    } else {
        return errors.New("nil order")
    }
}

// AFTER: guard clauses — happy path is unindented
func process(order *Order) error {
    if order == nil {
        return errors.New("nil order")
    }
    if !order.IsValid() {
        return errors.New("invalid order")
    }
    if order.Total <= 0 {
        return errors.New("zero total")
    }
    return charge(order)
}
```

### Replace Type Switch Duplication with Interface

```go
// BEFORE: repeated type switches
func area(s Shape) float64 {
    switch s := s.(type) {
    case Circle:
        return math.Pi * s.Radius * s.Radius
    case Rectangle:
        return s.Width * s.Height
    }
    return 0
}

func perimeter(s Shape) float64 {
    switch s := s.(type) {
    case Circle:
        return 2 * math.Pi * s.Radius
    case Rectangle:
        return 2 * (s.Width + s.Height)
    }
    return 0
}

// AFTER: behavior on the type
type Shape interface {
    Area() float64
    Perimeter() float64
}
```

Only do this when the same type switch appears in **multiple** places. A single type switch is fine.

### Replace Boolean Flags with Separate Functions

```go
// BEFORE: flag parameter
func fetch(url string, useCache bool) (*Response, error)

// AFTER: two clear functions
func Fetch(url string) (*Response, error)
func FetchCached(url string) (*Response, error)
```

---

## Concurrency Refactoring

### Extract Goroutine Logic into Named Functions

```go
// BEFORE: anonymous closure soup
go func() {
    for {
        select {
        case msg := <-ch:
            // 30 lines of processing
        case <-ctx.Done():
            return
        }
    }
}()

// AFTER: named, testable function
go s.processMessages(ctx, ch)

func (s *Service) processMessages(ctx context.Context, ch <-chan Message) {
    for {
        select {
        case msg := <-ch:
            s.handleMessage(msg)
        case <-ctx.Done():
            return
        }
    }
}
```

### Extract Channel Setup into Constructor

```go
// BEFORE: channels created and wired ad-hoc in main
func main() {
    ch := make(chan Event, 100)
    done := make(chan struct{})
    // ... complex wiring
}

// AFTER: encapsulated in a constructor
type Pipeline struct {
    events chan Event
    done   chan struct{}
}

func NewPipeline(bufSize int) *Pipeline {
    return &Pipeline{
        events: make(chan Event, bufSize),
        done:   make(chan struct{}),
    }
}
```

### Replace sync.Mutex with Channel When Logic Grows

When a mutex-protected section grows beyond simple get/set into a state machine, refactor to the for-select-loop pattern:

```go
// BEFORE: mutex protecting complex state transitions
func (s *Service) HandleEvent(e Event) {
    s.mu.Lock()
    defer s.mu.Unlock()
    switch s.state {
    case StateIdle:
        if e.Type == "start" {
            s.state = StateRunning
            s.startTime = time.Now()
            // ... more logic
        }
    // ... more states
    }
}

// AFTER: for-select loop owns the state
func (s *Service) run(ctx context.Context) {
    state := StateIdle
    var startTime time.Time
    for {
        select {
        case e := <-s.events:
            switch state {
            case StateIdle:
                if e.Type == "start" {
                    state = StateRunning
                    startTime = time.Now()
                }
            }
        case <-ctx.Done():
            return
        }
    }
}
```

### Separate Pure Logic from Concurrent Orchestration

```go
// BEFORE: business logic tangled with concurrency
func (s *Service) Process(ctx context.Context, items []Item) ([]Result, error) {
    var mu sync.Mutex
    var results []Result
    g, ctx := errgroup.WithContext(ctx)
    for _, item := range items {
        item := item
        g.Go(func() error {
            // 20 lines of business logic mixed with locking
            transformed := transform(item)
            validated := validate(transformed)
            mu.Lock()
            results = append(results, validated)
            mu.Unlock()
            return nil
        })
    }
    // ...
}

// AFTER: pure logic extracted, concurrency is just orchestration
func processItem(item Item) (Result, error) {
    transformed := transform(item)
    return validate(transformed)
}

func (s *Service) ProcessAll(ctx context.Context, items []Item) ([]Result, error) {
    g, ctx := errgroup.WithContext(ctx)
    results := make([]Result, len(items))
    for i, item := range items {
        i, item := i, item
        g.Go(func() error {
            r, err := processItem(item)
            if err != nil {
                return err
            }
            results[i] = r // no mutex needed — distinct indices
            return nil
        })
    }
    if err := g.Wait(); err != nil {
        return nil, err
    }
    return results, nil
}
```

---

## Package Refactoring

### Split Bloated Packages

Signs a package needs splitting:

- The package has multiple unrelated types
- You import the package for one type but get 20 others
- The package name is vague: `util`, `common`, `helpers`

```
// BEFORE
internal/
  util/
    http.go      // HTTP helpers
    strings.go   // string helpers
    errors.go    // error types
    config.go    // config parsing

// AFTER
internal/
  httputil/
    httputil.go
  config/
    config.go
  (strings and errors inlined where used, or kept if shared)
```

### Collapse Unnecessary Package Nesting

```
// BEFORE: over-engineered
internal/
  services/
    agent/
      service/
        agent_service.go

// AFTER: flat and clear
internal/
  agent/
    agent.go
```

### Move Function to Where the Data Lives

```go
// BEFORE: feature envy — format lives in handler, uses only User fields
// internal/handler/user.go
func formatUserDisplay(u *user.User) string {
    return fmt.Sprintf("%s (%s)", u.Name, u.Email)
}

// AFTER: method on the type that owns the data
// internal/user/user.go
func (u *User) DisplayString() string {
    return fmt.Sprintf("%s (%s)", u.Name, u.Email)
}
```

---

## Test Refactoring — Go Idioms

### Table-Driven Tests

When you see multiple test functions with the same structure but different inputs:

```go
// BEFORE: repetitive test functions
func TestParseToken_Valid(t *testing.T) { ... }
func TestParseToken_Expired(t *testing.T) { ... }
func TestParseToken_Malformed(t *testing.T) { ... }

// AFTER: table-driven
func TestParseToken(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *Token
        wantErr string
    }{
        {name: "valid", input: "abc.def.ghi", want: &Token{Sub: "user1"}},
        {name: "expired", input: "exp.ire.d", wantErr: "token expired"},
        {name: "malformed", input: "garbage", wantErr: "invalid token"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseToken(tt.input)
            if tt.wantErr != "" {
                require.ErrorContains(t, err, tt.wantErr)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Extract Test Helpers

```go
// BEFORE: setup duplicated across tests
func TestServiceA(t *testing.T) {
    db := setupTestDB(t)
    store := NewPostgresStore(db)
    svc := NewService(store, logger)
    // ...
}

// AFTER: helper function
func newTestService(t *testing.T) *Service {
    t.Helper()
    store := &fakeStore{}
    return NewService(store, slog.Default())
}

func TestServiceA(t *testing.T) {
    svc := newTestService(t)
    // ...
}
```

### Replace Mocks with Fakes

```go
// BEFORE: mock with expectations (brittle, tests implementation)
mock.EXPECT().Save(gomock.Any()).Return(nil).Times(1)

// AFTER: fake with real behavior (tests behavior)
type fakeStore struct {
    items map[string]*Item
}

func (f *fakeStore) Get(_ context.Context, id string) (*Item, error) {
    item, ok := f.items[id]
    if !ok {
        return nil, ErrNotFound
    }
    return item, nil
}

func (f *fakeStore) Save(_ context.Context, item *Item) error {
    f.items[item.ID] = item
    return nil
}
```

---

## Code Smells — Go-Specific Detection and Response

| Smell                                                     | Detection                                           | Action                                                             |
| --------------------------------------------------------- | --------------------------------------------------- | ------------------------------------------------------------------ |
| `interface{}` / `any` everywhere                          | Loss of type safety, runtime panics                 | Introduce generic or concrete types                                |
| `init()` functions                                        | Hidden side effects, test ordering issues           | Move to explicit initialization in `main` or constructors          |
| Package-level `var` (mutable)                             | Global state, test interference                     | Encapsulate in a struct, inject via constructor                    |
| `panic` in library code                                   | Crashes caller, no recovery path                    | Return `error` instead                                             |
| Stuttering names                                          | `user.UserService`, `http.HTTPClient`               | `user.Service`, `http.Client`                                      |
| `util` / `common` / `helpers` packages                    | Grab-bag with no cohesion                           | Split by domain or inline                                          |
| Empty interface parameters                                | `func Do(opts interface{})`                         | Define a concrete `Options` struct                                 |
| Check-and-act without lock                                | `if m[k] == nil { m[k] = v }`                       | Use mutex or `sync.Map.LoadOrStore`                                |
| Error string starts with capital or ends with punctuation | Breaks `fmt.Errorf("x: %w", err)` chaining          | Lowercase, no trailing period                                      |
| Goroutine without shutdown path                           | Goroutine leak                                      | Accept `context.Context` or `done` channel                         |
| `sync.Mutex` in API surface                               | Forces callers into lock discipline                 | Encapsulate behind methods                                         |
| Deep package nesting                                      | `internal/services/agent/service/impl/`             | Flatten to `internal/agent/`                                       |
| Returning concrete where interface suffices               | Tight coupling to implementation                    | Return interface at package boundary (when consumers are external) |
| `select {}` without comment                               | Unclear intent: deliberate block or forgotten case? | Add comment or replace with proper signal wait                     |

---

## Architecture Rules

- **Design Stamina Hypothesis**: good internal design pays for itself in development speed over time.
- **YAGNI**: build for current needs. Refactoring is cheaper than speculative flexibility.
- **Branch by Abstraction**: for large refactorings, introduce an interface, migrate callers gradually, remove the old implementation.
- **Parallel Change (expand-contract)**: for APIs, add the new function alongside the old, migrate consumers, deprecate and remove the old.
- Long-lived branches make refactoring dangerous. Integrate continuously.
- **Camping rule**: always leave the codebase healthier than you found it.
- **A little copying is better than a little dependency**: within a module, extract shared code. Across module/service boundaries, prefer duplication over coupling.

---

## Testing Rules for Refactoring

- Before refactoring, ensure tests exist. If they don't, write them first.
- TDD cycle: write a failing test, implement minimum code to pass, refactor while green.
- Fresh fixture per test. Use `t.Cleanup()` for teardown. Never share mutable state between tests.
- Structure: arrange, act, assert.
- Test **behavior**, not implementation. Tests coupled to implementation break on every refactor.
- Focus on risky and complex areas. Don't chase 100% coverage.
- Probe boundaries: nil slices, empty strings, zero values, context cancellation, error paths.
- Bug found? Write the exposing test first, then fix.
- Hard-to-test code = bad design. The test is feedback, not the problem.
- Use `-race` flag always: `go test -race ./...`
- Use `testify/assert` and `testify/require` for clear assertions.

---

## Decision Framework

When making a structural decision, ask in order:

1. **Is it testable?** Can I write a fast, deterministic test in isolation? If not, redesign.
2. **Are concerns separated?** Is domain logic free of infrastructure? If not, extract.
3. **Is it modular?** Can I understand this unit without reading the rest of the system? If not, decompose.
4. **Is coupling minimal?** Does changing this force changes elsewhere? If so, introduce an interface.
5. **Is it the simplest thing that works?** Am I adding complexity for hypothetical futures? If so, remove it.
6. **Is it idiomatic Go?** Would a Go developer reading this for the first time find it natural? If not, align with conventions.

When debugging during refactoring:

1. State the problem: expected vs. actual.
2. Form a conjecture.
3. Design an experiment that would **refute** the conjecture.
4. Run it. If refuted, form a better conjecture. If confirmed, act on it.
