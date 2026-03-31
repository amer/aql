# Advanced Go Concurrency Patterns

A comprehensive reference covering advanced concurrency patterns, the Go memory model, context-based cancellation, channel API design, and extended sync primitives. Based on resources from [go.dev/wiki/LearnConcurrency](https://go.dev/wiki/LearnConcurrency#advanced).

---

## Table of Contents

- [1. The Go Memory Model](#1-the-go-memory-model)
- [2. Core Patterns](#2-core-patterns)
  - [2.1 The For-Select Loop](#21-the-for-select-loop)
  - [2.2 Nil Channels for Dynamic Case Control](#22-nil-channels-for-dynamic-case-control)
  - [2.3 Service Channels with Reply Channels](#23-service-channels-with-reply-channels)
  - [2.4 Ping-Pong](#24-ping-pong)
- [3. Composition Patterns](#3-composition-patterns)
  - [3.1 Fan-In (Merge)](#31-fan-in-merge)
  - [3.2 Subscription Service](#32-subscription-service)
  - [3.3 Deduplication Decorator](#33-deduplication-decorator)
- [4. Context: Cancellation, Deadlines, and Values](#4-context-cancellation-deadlines-and-values)
  - [4.1 The Context Interface](#41-the-context-interface)
  - [4.2 Deriving Contexts](#42-deriving-contexts)
  - [4.3 HTTP Request Cancellation](#43-http-request-cancellation)
  - [4.4 Request-Scoped Values](#44-request-scoped-values)
  - [4.5 Best Practices and Anti-Patterns](#45-best-practices-and-anti-patterns)
- [5. Extended Sync Primitives](#5-extended-sync-primitives)
  - [5.1 singleflight](#51-singleflight)
  - [5.2 errgroup](#52-errgroup)
  - [5.3 Bounded Concurrency (Semaphores)](#53-bounded-concurrency-semaphores)
  - [5.4 Weighted Semaphores](#54-weighted-semaphores)
- [6. Channel API Design Principles](#6-channel-api-design-principles)
- [7. The Go Memory Model In-Depth](#7-the-go-memory-model-in-depth)
  - [7.1 Happens-Before Guarantees](#71-happens-before-guarantees)
  - [7.2 Channel Synchronization Rules](#72-channel-synchronization-rules)
  - [7.3 Locks, Once, and Atomics](#73-locks-once-and-atomics)
  - [7.4 Incorrect Synchronization Patterns](#74-incorrect-synchronization-patterns)
- [8. Common Anti-Patterns and Bugs](#8-common-anti-patterns-and-bugs)
- [9. Debugging Tools](#9-debugging-tools)
- [Resources](#resources)

---

## 1. The Go Memory Model

The Go memory model's core guarantee is **DRF-SC**: data-race-free programs execute in a sequentially consistent manner. Programs with data races have undefined behavior.

> "If you must read the rest of this document to understand the behavior of your program, you are being too clever. Don't be clever."

The cardinal rule: programs that modify data accessed by multiple goroutines **must serialize access** using channels or synchronization primitives (`sync`, `sync/atomic`).

---

## 2. Core Patterns

### 2.1 The For-Select Loop

The fundamental building block for concurrent services in Go. A single goroutine owns mutable state and communicates via channels in a select loop.

```go
func (s *service) loop() {
    // declare mutable state
    var pending []Item
    var err error

    for {
        // set up channels for this iteration
        var updates chan Item
        if len(pending) > 0 {
            updates = s.out // enable send case
        }

        select {
        case item := <-s.incoming:
            pending = append(pending, item)
        case updates <- pending[0]:
            pending = pending[1:]
        case errc := <-s.closing:
            errc <- err
            return
        }
    }
}
```

**Key principles:**

1. The goroutine serializes access to local mutable state — no locks needed.
2. Each select case reads/writes that local state.
3. Channels convey data, timer events, and cancellation signals.

### 2.2 Nil Channels for Dynamic Case Control

Sends and receives on nil channels **block forever**. A `select` never picks a blocking case. This lets you enable/disable cases dynamically:

```go
var startFetch <-chan time.Time
if fetchDone == nil && len(pending) < maxPending {
    startFetch = time.After(fetchDelay) // enable fetch case
}

var first Item
var updates chan Item // nil — send disabled
if len(pending) > 0 {
    first = pending[0]
    updates = s.updates // enable send case
}

select {
case <-startFetch:
    // start a fetch
case updates <- first:
    pending = pending[1:]
case errc := <-s.closing:
    errc <- err
    return
}
```

This replaces complex boolean flags with channel nil/non-nil state.

### 2.3 Service Channels with Reply Channels

Use `chan chan error` (or `chan chan T`) to send a request and receive a reply through the same goroutine loop:

```go
type sub struct {
    closing chan chan error // request channel carrying a reply channel
}

func (s *sub) Close() error {
    errc := make(chan error)
    s.closing <- errc   // send request (with reply channel)
    return <-errc       // block until reply
}

// Inside the for-select loop:
select {
case errc := <-s.closing:
    errc <- err          // send reply
    close(s.updates)
    return
// ... other cases
}
```

This pattern provides request-response semantics without locks or condition variables.

### 2.4 Ping-Pong

A minimal demonstration of channel-based communication between goroutines:

```go
type Ball struct{ hits int }

func main() {
    table := make(chan *Ball)
    go player("ping", table)
    go player("pong", table)
    table <- new(Ball)           // game on; toss the ball
    time.Sleep(1 * time.Second)
    <-table                      // game over; grab the ball
}

func player(name string, table chan *Ball) {
    for {
        ball := <-table
        ball.hits++
        fmt.Println(name, ball.hits)
        time.Sleep(100 * time.Millisecond)
        table <- ball
    }
}
```

The channel enforces turn-taking — only one goroutine holds the ball at a time.

---

## 3. Composition Patterns

### 3.1 Fan-In (Merge)

Combines multiple subscription streams into one. Each input subscription gets its own forwarding goroutine, and a quit channel enables clean shutdown:

```go
type merge struct {
    subs    []Subscription
    updates chan Item
    quit    chan struct{}
    errs    chan error
}

func Merge(subs ...Subscription) Subscription {
    m := &merge{
        subs:    subs,
        updates: make(chan Item),
        quit:    make(chan struct{}),
        errs:    make(chan error),
    }
    for _, sub := range subs {
        go func(s Subscription) {
            for {
                var it Item
                select {
                case it = <-s.Updates():
                case <-m.quit:
                    m.errs <- s.Close()
                    return
                }
                select {
                case m.updates <- it:
                case <-m.quit:
                    m.errs <- s.Close()
                    return
                }
            }
        }(sub)
    }
    return m
}

func (m *merge) Close() (err error) {
    close(m.quit) // signal all forwarding goroutines
    for range m.subs {
        if e := <-m.errs; e != nil {
            err = e
        }
    }
    close(m.updates)
    return
}
```

**Critical detail**: Each loop iteration uses **two** select statements — one for receive, one for send — both checking the quit channel. A naive version (`m.updates <- it` without select) blocks forever if the receiver stops reading.

### 3.2 Subscription Service

A complete concurrent service that fetches items on a schedule, deduplicates, buffers, and streams them to a consumer:

```go
type Fetcher interface {
    Fetch() (items []Item, next time.Time, err error)
}

type Subscription interface {
    Updates() <-chan Item
    Close() error
}

func Subscribe(fetcher Fetcher) Subscription {
    s := &sub{
        fetcher: fetcher,
        updates: make(chan Item),
        closing: make(chan chan error),
    }
    go s.loop()
    return s
}

func (s *sub) loop() {
    const maxPending = 10
    type fetchResult struct {
        fetched []Item
        next    time.Time
        err     error
    }

    var fetchDone chan fetchResult // nil when no fetch in progress
    var pending []Item
    var next time.Time
    var err error
    seen := make(map[string]bool)

    for {
        // Calculate fetch delay
        var fetchDelay time.Duration
        if now := time.Now(); next.After(now) {
            fetchDelay = next.Sub(now)
        }

        // Enable fetch only when idle and not overloaded
        var startFetch <-chan time.Time
        if fetchDone == nil && len(pending) < maxPending {
            startFetch = time.After(fetchDelay)
        }

        // Enable send only when there are pending items
        var first Item
        var updates chan Item
        if len(pending) > 0 {
            first = pending[0]
            updates = s.updates
        }

        select {
        case errc := <-s.closing:
            errc <- err
            close(s.updates)
            return

        case <-startFetch:
            fetchDone = make(chan fetchResult, 1)
            go func() {
                fetched, next, err := s.fetcher.Fetch()
                fetchDone <- fetchResult{fetched, next, err}
            }()

        case result := <-fetchDone:
            fetchDone = nil
            next, err = result.next, result.err
            if err != nil {
                next = time.Now().Add(10 * time.Second)
                break
            }
            for _, item := range result.fetched {
                if !seen[item.GUID] {
                    pending = append(pending, item)
                    seen[item.GUID] = true
                }
            }

        case updates <- first:
            pending = pending[1:]
        }
    }
}
```

This demonstrates multiple advanced techniques working together:

- **Nil channel toggling** for backpressure and fetch gating
- **Async fetch** with result channel
- **Reply channel** for clean shutdown
- **Bounded buffering** with `maxPending`

### 3.3 Deduplication Decorator

Wraps a Subscription to filter out duplicate items, using the nil channel technique for flow control:

```go
func Dedupe(s Subscription) Subscription {
    d := &deduper{
        s:       s,
        updates: make(chan Item),
        closing: make(chan chan error),
    }
    go d.loop()
    return d
}

func (d *deduper) loop() {
    in := d.s.Updates()
    var pending Item
    var out chan Item // nil — send disabled
    seen := make(map[string]bool)

    for {
        select {
        case it := <-in:
            if !seen[it.GUID] {
                pending = it
                in = nil        // disable receive until sent
                out = d.updates // enable send
                seen[it.GUID] = true
            }
        case out <- pending:
            in = d.s.Updates() // re-enable receive
            out = nil          // disable send
        case errc := <-d.closing:
            errc <- d.s.Close()
            close(d.updates)
            return
        }
    }
}
```

Composable with Merge and Subscribe:

```go
merged := Merge(
    Dedupe(Subscribe(Fetch("blog.golang.org"))),
    Dedupe(Subscribe(Fetch("googleblog.blogspot.com"))),
)
```

---

## 4. Context: Cancellation, Deadlines, and Values

### 4.1 The Context Interface

```go
type Context interface {
    Done() <-chan struct{}              // closed when canceled or timed out
    Err() error                        // why it was canceled
    Deadline() (time.Time, bool)       // when it will be canceled, if set
    Value(key interface{}) interface{} // request-scoped data
}
```

- Safe for simultaneous use by multiple goroutines.
- No `Cancel` method — the receiver can only observe cancellation, never trigger it.
- Values form a tree: child contexts inherit and can shadow parent values.

### 4.2 Deriving Contexts

```go
// Root — never canceled, no deadline, no values
ctx := context.Background()

// With manual cancellation
ctx, cancel := context.WithCancel(parent)
defer cancel()

// With automatic timeout
ctx, cancel := context.WithTimeout(parent, 5*time.Second)
defer cancel()

// With absolute deadline
ctx, cancel := context.WithDeadline(parent, time.Now().Add(5*time.Second))
defer cancel()

// With a request-scoped value
ctx = context.WithValue(parent, myKey, myValue)
```

**Always `defer cancel()`** — even if the context times out naturally, calling cancel releases timer resources.

### 4.3 HTTP Request Cancellation

```go
func handleSearch(w http.ResponseWriter, req *http.Request) {
    var ctx context.Context
    var cancel context.CancelFunc

    timeout, err := time.ParseDuration(req.FormValue("timeout"))
    if err == nil {
        ctx, cancel = context.WithTimeout(context.Background(), timeout)
    } else {
        ctx, cancel = context.WithCancel(context.Background())
    }
    defer cancel()

    results, err := search(ctx, req.FormValue("q"))
    // ...
}
```

Performing an HTTP call that respects context cancellation:

```go
func httpDo(ctx context.Context, req *http.Request, f func(*http.Response, error) error) error {
    c := make(chan error, 1)
    req = req.WithContext(ctx)

    go func() {
        c <- f(http.DefaultClient.Do(req))
    }()

    select {
    case <-ctx.Done():
        <-c // wait for goroutine to finish
        return ctx.Err()
    case err := <-c:
        return err
    }
}
```

### 4.4 Request-Scoped Values

Define unexported key types to prevent collisions across packages:

```go
// Package userip
type key int

const userIPKey key = 0

func NewContext(ctx context.Context, ip net.IP) context.Context {
    return context.WithValue(ctx, userIPKey, ip)
}

func FromContext(ctx context.Context) (net.IP, bool) {
    ip, ok := ctx.Value(userIPKey).(net.IP)
    return ip, ok
}
```

### 4.5 Best Practices and Anti-Patterns

**Do:**

- Pass `context.Context` as the first parameter to every function on the call path.
- Always `defer cancel()`.
- Check `ctx.Deadline()` to decide whether starting work is worthwhile.
- Use `Value()` only for request-scoped data (auth tokens, request IDs, trace IDs).

**Don't:**

- Store `Context` in a struct field — pass it as a parameter.
- Put application-lifetime values in Context — it's for request scope only.
- Ignore cancellation signals in goroutines — check `<-ctx.Done()` and return promptly.

---

## 5. Extended Sync Primitives

### 5.1 singleflight

Deduplicates concurrent calls to the same expensive operation. Multiple goroutines requesting the same key execute the function once, and all receive the same result.

```go
import "golang.org/x/sync/singleflight"

var group singleflight.Group

func FetchWeather(city string) (*Info, error) {
    result, err, _ := group.Do(city, func() (interface{}, error) {
        return fetchWeatherFromDB(city)
    })
    if err != nil {
        return nil, fmt.Errorf("weather.City %s: %w", city, err)
    }
    return result.(*Info), nil
}
```

Ideal for cache population, database lookups, or any idempotent operation where concurrent duplicate requests are wasteful.

### 5.2 errgroup

An enhanced `sync.WaitGroup` that propagates the first error from any goroutine:

```go
import "golang.org/x/sync/errgroup"

func FetchAll(cities ...string) ([]*Info, error) {
    g, ctx := errgroup.WithContext(context.Background())
    results := make([]*Info, len(cities))

    for i, city := range cities {
        i, city := i, city // capture loop variables
        g.Go(func() error {
            info, err := FetchWeather(city)
            if err != nil {
                return err
            }
            results[i] = info // safe: each goroutine writes to its own index
            return nil
        })
    }

    if err := g.Wait(); err != nil {
        return nil, err
    }
    return results, nil
}
```

`errgroup.WithContext` derives a context that's canceled when any goroutine returns an error, allowing others to bail out early.

### 5.3 Bounded Concurrency (Semaphores)

Use a buffered channel as a counting semaphore to limit concurrent goroutines:

```go
sem := make(chan struct{}, 10) // at most 10 concurrent operations

for _, task := range tasks {
    task := task
    sem <- struct{}{} // acquire — blocks when 10 goroutines are running
    go func() {
        defer func() { <-sem }() // release
        process(task)
    }()
}

// Wait for all to complete
for i := 0; i < cap(sem); i++ {
    sem <- struct{}{}
}
```

### 5.4 Weighted Semaphores

For tasks with variable cost, use `golang.org/x/sync/semaphore`:

```go
import "golang.org/x/sync/semaphore"

sem := semaphore.NewWeighted(100) // total capacity 100

for _, city := range cities {
    cost := int64(len(city)) // variable cost per task
    if err := sem.Acquire(ctx, cost); err != nil {
        break // context canceled
    }
    go func(c string, cost int64) {
        defer sem.Release(cost)
        processCity(c)
    }(city, cost)
}

// Wait for all in-flight work to complete
sem.Acquire(ctx, 100)
```

---

## 6. Channel API Design Principles

Five rules for designing public Go APIs that use channels (from [Alan Shreve's article](https://inconshreveable.com/07-08-2014/principles-of-designing-go-apis-with-channels/)):

### Principle 1: Always Declare Channel Direction

The compiler enforces directionality and it documents data flow:

```go
func After(d Duration) <-chan Time         // receive-only
func Notify(c chan<- os.Signal, sig ...os.Signal)  // send-only
```

### Principle 2: Document Slow-Consumer Behavior for Unbounded Streams

When an API sends an unlimited number of values, it **must** document what happens when the receiver is slow:

- **Drop values** (e.g., `signal.Notify` drops silently)
- **Adjust timing** (e.g., `time.NewTicker` adjusts intervals)
- **Block** (e.g., `ssh.Conn.OpenChannel` blocks the connection)

### Principle 3: Document Buffer-Exhaustion Behavior for Bounded Streams on Passed Channels

When an API writes a bounded number of values into a channel the caller provided, document what happens when the buffer fills (drop, block, or error).

### Principle 4: Accept Channels for Unbounded Streams (Don't Return Them)

Let the caller control buffer size and reuse channels across multiple subscriptions:

```go
// Good: caller controls the channel
func Notify(c chan<- os.Signal, sig ...os.Signal)

// Less flexible: API controls the channel
func Signals(sig ...os.Signal) <-chan os.Signal
```

Accepting channels enables multiplexing multiple subscriptions onto one channel:

```go
msgs := make(chan Msg, 128)
Subscribe("topic-a", msgs)
Subscribe("topic-b", msgs) // same channel, no extra goroutine needed
for m := range msgs {
    handle(m)
}
```

### Principle 5: Return Buffered Channels for Bounded Streams

When an API sends a known, small number of values, returning an appropriately buffered channel is safe and ergonomic:

```go
// Sends exactly one value — buffer of 1 is safe
func After(d Duration) <-chan Time

// Sends exactly one value
type CloseNotifier interface {
    CloseNotify() <-chan bool
}
```

---

## 7. The Go Memory Model In-Depth

### 7.1 Happens-Before Guarantees

The happens-before relation is the transitive closure of:

1. **Sequenced-before**: program order within a goroutine.
2. **Synchronized-before**: established by synchronization operations.

A read `r` of variable `x` observes a write `w` if:

- `w` happens-before `r`
- No other write to `x` happens between `w` and `r`

### 7.2 Channel Synchronization Rules

| Operation                                           | Guarantee                                               |
| --------------------------------------------------- | ------------------------------------------------------- |
| Send on channel                                     | Synchronized-before the corresponding receive completes |
| Close of channel                                    | Synchronized-before a receive that returns zero value   |
| Receive from **unbuffered** channel                 | Synchronized-before the corresponding send completes    |
| k-th receive from **buffered** channel (capacity C) | Synchronized-before the (k+C)-th send completes         |

The unbuffered channel rule is surprising — the **receive** happens-before the **send completes**:

```go
var c = make(chan int) // unbuffered
var a string

func f() {
    a = "hello, world"
    <-c                  // receive happens first
}

func main() {
    go f()
    c <- 0               // send completes after receive
    print(a)             // guaranteed: "hello, world"
}
```

With a **buffered** channel of capacity 1, this guarantee **does not hold** — the send can complete before the receive.

### 7.3 Locks, Once, and Atomics

**Mutex**: The n-th `Unlock()` is synchronized-before the (n+1)-th `Lock()` returns.

**sync.Once**: The single execution of `f()` in `once.Do(f)` is synchronized-before any `once.Do(f)` call returns.

```go
var a string
var once sync.Once

func setup() { a = "hello, world" }

func doprint() {
    once.Do(setup)
    print(a) // guaranteed: "hello, world"
}
```

**Atomics**: If the effect of atomic operation A is observed by atomic operation B, then A is synchronized-before B.

### 7.4 Incorrect Synchronization Patterns

**Double-checked locking (broken)**:

```go
var a string
var done bool

func setup() {
    a = "hello, world"
    done = true
}

func doprint() {
    if !done {       // no synchronization!
        once.Do(setup)
    }
    print(a)         // NOT guaranteed to see "hello, world"
}
```

Observing `done == true` does **not** imply observing `a == "hello, world"`. Without synchronization, the compiler and hardware may reorder the writes.

**Busy-waiting (broken)**:

```go
var a string
var done bool

func main() {
    go func() {
        a = "hello"
        done = true
    }()
    for !done {
        // spin — may never terminate!
    }
    print(a) // no guarantee
}
```

The compiler can hoist the read of `done` out of the loop (since nothing synchronizes), turning it into an infinite loop.

**Fix**: Always use proper synchronization — channels, mutexes, `sync.Once`, or `sync/atomic`.

---

## 8. Common Anti-Patterns and Bugs

### Bug: Unsynchronized Shared State

```go
// WRONG: race condition between loop() and Close()
func (s *naiveSub) loop() {
    for {
        if s.closed { ... }    // unsynchronized read
        s.err = err            // unsynchronized write
    }
}

func (s *naiveSub) Close() error {
    s.closed = true            // unsynchronized write
    return s.err               // unsynchronized read
}
```

**Fix**: Use the for-select loop with a closing channel.

### Bug: Blocking on Sleep

```go
// WRONG: time.Sleep prevents handling Close
if err != nil {
    time.Sleep(10 * time.Second) // blocks entire loop
    continue
}
```

**Fix**: Use `time.After` as a select case — the loop remains responsive to other channels.

### Bug: Blocking on Send Without Quit Check

```go
// WRONG: blocks forever if receiver stops reading
for _, item := range items {
    s.updates <- item
}
```

**Fix**: Always pair sends with a quit/cancel check via select:

```go
select {
case s.updates <- item:
case <-s.quit:
    return
}
```

### Bug: Goroutine Leak in Naive Merge

```go
// WRONG: forwarding goroutines block forever when Close is called
go func(s Subscription) {
    for it := range s.Updates() {
        m.updates <- it // blocks if nobody reads
    }
}(sub)
```

**Fix**: Check a quit channel in both the receive and send select cases (see the [Fan-In pattern](#31-fan-in-merge)).

---

## 9. Debugging Tools

### Race Detector

```bash
go test -race ./...
go run -race main.go
```

Detects unsynchronized memory access at runtime. Use it in CI and during development.

### Deadlock Detection

Go's runtime detects when **all** goroutines are blocked and panics with a full goroutine dump:

```
fatal error: all goroutines are asleep - deadlock!
```

### Stack Traces

For debugging, intentionally dump all goroutine stacks:

```go
import "runtime/debug"
debug.PrintStack()
```

Or use `SIGQUIT` (Ctrl+\\) to dump stacks of a running program.

---

## Resources

| Resource                                                                                                                               | Author         | Type        |
| -------------------------------------------------------------------------------------------------------------------------------------- | -------------- | ----------- |
| [Advanced Go Concurrency Patterns](https://go.dev/blog/advanced-go-concurrency-patterns)                                               | Andrew Gerrand | Blog + Talk |
| [Advanced Go Concurrency Patterns (Slides)](https://go.dev/talks/2013/advconc.slide)                                                   | Sameer Ajmani  | Slides      |
| [Go Concurrency Patterns: Context](https://go.dev/blog/context)                                                                        | Sameer Ajmani  | Blog        |
| [The Go Memory Model](https://go.dev/ref/mem)                                                                                          | Go Team        | Reference   |
| [Package sync/atomic](https://pkg.go.dev/sync/atomic/)                                                                                 | Go Team        | Reference   |
| [Principles of Designing Go APIs with Channels](https://inconshreveable.com/07-08-2014/principles-of-designing-go-apis-with-channels/) | Alan Shreve    | Blog        |
| [Advanced Go Concurrency Primitives](https://encore.dev/blog/advanced-go-concurrency)                                                  | Encore         | Blog        |
| [The Scheduler Saga](https://www.youtube.com/watch?v=YHRO5WQGh0k)                                                                      | Kavya Joshi    | Talk        |
| [Understanding Channels](https://www.youtube.com/watch?v=KBZlN0izeiY)                                                                  | Kavya Joshi    | Talk        |
