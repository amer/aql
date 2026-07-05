# Architecture Refactor Plan

**Date:** 2026-07-05
**Status:** Proposed — handoff document for implementing agent
**Audit basis:** Four parallel architecture audits (TUI, agent core, boundaries, tests), all claims spot-verified against source at commit `c704943`.

---

## How to execute this plan (non-negotiable)

1. **TDD every step.** Write the failing test first. Run it. Confirm it fails **for the predicted reason**. Then implement the minimum to pass. Then refactor. Then commit.
2. **One logical change per commit**, conventional-commit format. The phases below are ordered so each commit leaves the tree green.
3. **Verification loop after every change:** `go test ./...` then `go vet ./...`. Before finishing a phase: `go test -race -count=1 ./...`.
4. **Do not batch phases.** If a step's test fails for a reason you can't immediately explain, revert and take a smaller step.
5. Update file-guideline headers and `doc/` (architecture, adr, cli) when a step changes them — code is source of truth.

---

## Executive summary

The codebase's core design is sound: `internal/domain` is a clean port layer, `internal/llm` is a textbook adapter, `internal/stream` is a proper anti-corruption layer, and the tui/agent import boundary is honored. **Do not churn those.**

The problems cluster into five groups:

| #   | Group                     | Worst symptom                                                                                                                                                                    |
| --- | ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A   | **Live correctness bugs** | Sub-agents silently lose OAuth billing config; advertised tools ≠ dispatchable tools; `Run()` goroutine mutates shared agent state                                               |
| B   | **Boundary violations**   | `internal/models` is a second, parallel Anthropic adapter; Anthropic billing constants live in `domain`; agent transitively depends on the SDK                                   |
| C   | **Missing test seams**    | The production stream path (`ForwardWithHistory`), auto-compact pipeline, and `run()` wiring have 0% coverage; runner tests re-implement SSE encoding instead of faking the port |
| D   | **TUI God object**        | `tui.Model` owns 14+ concerns, `View()` is 112 lines and does syscalls per frame, ~25 test-only accessors exist because units aren't independently testable                      |
| E   | **Dead/duplicated code**  | Unused OAuth refresh, dead legacy render pipeline, duplicated OAuth request mutation in two adapters                                                                             |

**Order matters:** seams first (C enables TDD on everything else), then bugs (A), then boundaries (B — the structural fix makes part of A permanent), then TUI (D), then cleanup (E folds into the others).

---

## Phase 1 — Create the test seams (enables TDD for all later phases)

### 1.1 Promote a shared, scriptable `fakeChatClient`

**Problem.** Runner tests (~10 files) wire a real `llm.NewAnthropicClient` against `httptest` servers and hand-encode Anthropic SSE via `jsonToSSE()` (`internal/agent/testhelpers_test.go:38-102`, 26 `httptest.NewServer` usages). Agent tests are transitively coupled to `llm`'s SSE parsing — refactoring the adapter breaks tests that conceptually test the tool loop. A proper fake already exists but is private to `internal/agent/spawner_test.go:14-27`.

**Why.** The project's own rule: "Agent tests use fake ChatClient implementations." SSE coverage belongs in `internal/llm` (where `anthropic_test.go` already does it well). Every later phase needs this fake to write its red tests.

**What if we skip this?** Every Phase 2 test would need an httptest server + SSE encoding — 3x the test code, and Phase 3's adapter changes would break Phase 2's tests. Seam first is strictly cheaper.

**What if we make it too elaborate?** Keep it minimal: scripted multi-turn responses, captured `ChatParams` per call, injectable errors. No mock framework, no expectations DSL.

**Steps (TDD):**

1. RED: write a test in `internal/agent` that constructs the fake with two scripted turns and asserts captured params — fails to compile (fake doesn't exist yet at shared scope).
2. GREEN: move/extend `fakeChatClient` into `internal/agent/testhelpers_test.go` (multi-turn script, `CapturedParams []domain.ChatParams`, error injection).
3. REFACTOR: migrate **one** runner test file to the fake, confirm green. Migrate the rest incrementally (one commit each, or one commit for mechanical batches). Delete `jsonToSSE` when the last user is gone.
4. Commit(s): `test(agent): add shared scriptable fake ChatClient`, `refactor(agent): migrate runner tests off httptest SSE`.

### 1.2 Inject retry delay (clock seam)

**Problem.** `internal/agent/runner.go:165-170` sleeps real wall-clock (`time.After(base * 2^attempt)`); retry tests burn ~2s of the package's 4.6s runtime.

**Steps:** RED: test asserting retry uses injected delay func and completes in microseconds → GREEN: add `WithRetryDelay(func(attempt int) <-chan time.Time)` option (default: current behavior) → migrate `TestRunner_RetryOnTransient500` / `TestRunner_RetryExhausted`. Commit: `test(agent): inject retry delay for fast deterministic retry tests`.

### 1.3 Exec seam in `internal/agent/env.go`

**Problem.** `GitStatus` (30.8% covered) shells out to `git` with no seam; `detectShell()` spawns `sh -c "echo $SHELL"` to read an env var available via `os.Getenv`.

**Why.** `internal/diff/run.go` already shows the house pattern: injected exec func + one real-git integration test. Copy it.

**Steps:** RED: test `GitStatus` with a fake exec fn → GREEN: thread `execFn` parameter (same shape as `diff.NewRunner`) → replace `detectShell` with `os.Getenv("SHELL")` (RED first: test that no subprocess is spawned / value comes from env). Commits: `refactor(agent): inject exec seam into env collection`, `fix(agent): read SHELL from env instead of subprocess`.

### 1.4 Cover the production stream path

**Problem.** `stream.ForwardWithHistory` (`internal/stream/adapter.go:81`) is the **only** path production uses (`cmd/aql/main.go:187`) and has **zero** tests — all adapter tests exercise `Forward`, which production never calls. The history callbacks (`Append`/`Replace`) that architecture rule #2's race-freedom depends on are unverified. `agent.ReplaceHistory` is also 0%.

**What if a refactor breaks the history event protocol today?** The whole suite stays green while the app corrupts history. This is the single highest-value missing test in the repo.

**Steps:** RED: table test driving `ForwardWithHistory` with scripted `StreamEvent`s, asserting callback invocation order and forwarded TUI messages → GREEN (should pass immediately if code is correct — if it passes on first run, deliberately break the code to confirm the test can fail, then restore). Commit: `test(stream): cover ForwardWithHistory history callbacks`.

---

## Phase 2 — Fix the live correctness bugs

### 2.1 Sub-agents lose OAuth billing and max-tokens config — DONE

**Status (2026-07-05).** Fixed in `25f3f9f` and `0d5671f`. Implementation diverged from the plan in two ways:

- **Options propagation instead of a factory.** `Spawner` carries `agentOpts []Option` (set via `WithAgentOptions`), applied inherited-first with required wiring (`WithChatClient`, `WithToolExecutor`) appended last so it always wins. Same effect as the factory — new options propagate automatically — with one less abstraction.
- **A second fix was required in `main.go`.** Production wiring built its own `Spawner` directly, bypassing the default `NewToolExecutor` path, so the base options (`WithChatClient` + `WithOAuth`) had to be threaded into that spawner too via `WithAgentOptions(baseOpts...)`. Recorded in `doc/mistakes/003`.

RED confirmed as predicted (`OAuthBilling=false` on child calls); tests cover recursive inheritance to depth 2 (`TestAgent_SubAgentsInheritOAuth`) and `MaxTokens=16384` (`TestSpawner_WithAgentOptions_ChildCarriesOAuthBilling`).

**Problem.** `Spawn()` (`internal/agent/spawner.go:91-97`) constructs children with only `WithChatClient` + `WithToolExecutor` — never `WithOAuth()`. In OAuth mode (the primary supported auth mode, `main.go:139`), every sub-agent request omits the billing system-prompt header, beta headers, and thinking config, and uses 4096 max tokens instead of 16384. Likely 400/403s or silently degraded children.

**Why.** Duplicated construction: `Spawn()` re-implements the wiring knowledge that lives in `main.go`. Also note `Spawn()` builds the child `Spawner` as a struct literal, bypassing `NewSpawner` — a second duplication in the same function.

**What if we just add `isOAuth` to Spawner and pass `WithOAuth()` down?** Works today, but every future agent option becomes a new field Spawner must remember to forward — the same bug re-occurs on the next option. **Decision: inject an agent factory instead.** Spawner carries `newAgent func(cfg Config) (*Agent, error)` closed over the parent's full option set, built once in `main.go`/`agent.New`. New options propagate automatically.

**What if the deeper fix in 3.2 (OAuth → adapter construction) lands first?** Then the billing part of this bug vanishes structurally (children share `s.client`), leaving only max-tokens. We still want the factory — it fixes the _class_ of bug. Do 2.1 with the factory now; 3.2 later simplifies what the factory carries.

**Steps:**

1. RED: spawner test using the shared fake — spawn a child from an OAuth-configured parent, assert the child's captured `ChatParams` has `OAuthBilling=true` and `MaxTokens=16384`. Predict failure: `OAuthBilling=false`, `MaxTokens=4096`.
2. GREEN: add factory injection; `Spawn()` calls it; use `NewSpawner` for the child spawner.
3. Commit: `fix(agent): propagate parent agent options to spawned sub-agents`.

### 2.2 Advertised tools ≠ dispatchable tools

**Problem.** `buildChatParamsFrom` (`internal/agent/runner.go:276`) always advertises the static `tools.Definitions()` (all 15 tools), regardless of the executor's actual registry. Executors without `WithTaskStore` still advertise `task_create` → LLM calls it → `"unknown tool"` error. Sub-agents advertise `ask_user` they can't answer.

**Why.** Two unsynchronized sources of truth; hidden global registry the agent reaches into instead of a dependency. The existing `DispatchesAllKnownTools` test only guards the default configuration.

**What if we keep `Definitions()` and just filter?** Filtering by registry keys still leaves two structures to keep in sync (schema list vs handler map). **Decision: definitions become a property of the executor** — each `register*` call carries its `ToolDef`; the executor exposes `Defs() []domain.ToolDef`; the runner asks its injected executor.

**Steps:**

1. RED: construct an executor without task store, assert `Defs()` excludes task tools; runner test asserting `ChatParams.Tools` matches the executor's defs. Predict failure: `Defs` doesn't exist / all 15 advertised.
2. GREEN: move each def next to its handler registration; expose `Defs()`; runner uses it. Keep a compile-guard test that the default executor still exposes the full set (replaces the sync-by-discipline).
3. This also collapses the redundant entry points (`tools.Execute`, which builds a fresh TaskStore per call so `task_create`→`task_update` always fails; `tools.DefaultExecutor`; `agent.NewToolExecutor`'s variadic pseudo-option): delete `tools.Execute`, fold the rest into `NewExecutor` options. RED tests first for each deletion (confirm no production callers, migrate tests).
4. Commits: `refactor(tools): make tool definitions a property of the executor`, `refactor(tools): remove per-call executor construction in Execute`.

### 2.3 `Run()` goroutine mutates shared agent state

**Problem.** Every loop iteration inside the Run goroutine calls `RefreshClaudeMD()` (`runner.go:274`), which does file I/O and **writes** `a.claudeMD`, `a.claudeMDTime`, `a.systemPrompt` (`agent.go:196-213`) with no synchronization. This violates the codebase's own core invariant ("Run() never mutates agent state — the caller's goroutine owns it") that history carefully honors via snapshot+events. Also a CQS violation: `buildChatParamsFrom` is a query that performs I/O and mutation.

**What if we add a mutex?** Papering over an inverted ownership design; every future field needs remembering. **What if we drop hot-reload?** Losing a feature to fix a race is a last resort. **Decision: same pattern as history** — snapshot the system prompt at `Run()` start; hot-reload moves to the caller's goroutine (refresh before invoking `Run`, in the TUI submit path).

**Steps:**

1. RED: `-race` test — start `Run` with the fake client, concurrently call `SystemPrompt()` from the test goroutine; must fail under `-race` today. (If race detector doesn't trip deterministically, write the test to call `RefreshClaudeMD` from caller side while Run iterates — document observed failure.)
2. GREEN: snapshot prompt in `Run()`; move refresh call to caller (main.go submit path); `buildChatParamsFrom` takes the prompt as parameter.
3. Commit: `fix(agent): stop Run goroutine from mutating system prompt state`.

### 2.4 Unsafe live-agent swap on model switch

**Problem.** `*coder = *newCoder` (`cmd/aql/main.go:250`) copies an entire `Agent` over the live one from the TUI goroutine. If a stream is running, `Run()`'s goroutine reads `a.config`/`a.history` while every field is overwritten — a data race the single-writer convention doesn't cover. Also `configureTUI` takes 7 params including `**tea.Program` — over the project's own limit.

**What if we guard with "only swap when idle"?** Fragile — nothing enforces idleness. **Decision: the only thing that actually changes is the model — give `Agent` a `SetModel(string)`** applied via the same single-writer event discipline (or read via atomic/snapshot at Run start). Then introduce a small `app` struct in `cmd/aql` owning `program`, `cancel`, and the current agent, collapsing `configureTUI`'s parameter list.

**Steps:**

1. RED: test that model switch mid-run is race-free (`-race`, fake client with a slow scripted turn, switch model during it). Predict: race report on current code.
2. GREEN: `Agent.SetModel` + snapshot model at Run start; replace the struct copy.
3. REFACTOR: extract `app` struct; `configureTUI` params drop to ≤3. Separate commit.
4. Commits: `fix(cmd): replace whole-agent copy with SetModel on model switch`, `refactor(cmd): extract app struct owning program and agent wiring`.

### 2.5 Fire-and-forget goroutine in TUI answer handler

**Problem.** `handleAnswerQuestion` (`internal/tui/handlers.go:273`) spawns `go func() { responseCh <- answer }()` — unmanaged, leaks permanently if the agent never reads, and is invisible to the pure-update tests.

**Steps:** RED: test asserting the handler returns a `tea.Cmd` (not spawning directly) and the answer arrives → GREEN: `return m, func() tea.Msg { responseCh <- answer; return nil }`; make the channel buffered at creation. Commit: `fix(tui): deliver ask_user answer via tea.Cmd instead of raw goroutine`.

---

## Phase 3 — Repair the boundaries

### 3.1 Split `internal/models`: one Anthropic adapter, one port

**Problem.** `models/probe.go` builds its own `anthropic.Client` and makes raw SDK calls — a second, parallel Anthropic boundary beside `internal/llm`. Consequences already visible: the OAuth request mutation (billing header + thinking + betas) is duplicated verbatim (`llm/anthropic.go:191-203` vs `models/probe.go:206-217`); and because `agent` imports `models` (only for `ResolveModel`, a pure string function), the SDK lands in the agent's compile-time closure.

**Why.** One provider, one adapter. Two boundaries mean every provider change is a two-place fix, and adding a second provider requires editing `models` internals instead of adding an adapter.

**What if we merge `models` into `llm` wholesale?** No — `models` has genuinely pure logic (resolution, filtering, persistence, shortcuts) that must stay SDK-free. **Decision: split along purity.** Pure resolution/persistence stays in `models` (SDK-free); define a `ModelProber` port owned by the consumer (`models` or `domain`); implement it in `internal/llm` reusing the single client-construction path and the single `applyOAuthConfig`.

**What if `agent` still imports `models` after the split?** Fine — post-split `models` is stdlib-only, so the transitive SDK dependency disappears. Verify with an import test (see 3.5).

**Steps:**

1. RED: test that `internal/llm` implements the new `ModelProber` port against `httptest` (fixtures already exist in models tests — reuse).
2. GREEN: move `probeModel`/`buildClient` into `llm`; delete the duplicated OAuth block and the `probe.go:54-58` re-exports; `main.go` wires `llm` prober into `models.ProbeAndUpdate`.
3. Commits: `refactor(models): extract SDK probing behind ModelProber port in llm`, `refactor(llm): unify OAuth request mutation in one adapter`.

### 3.2 Purge Anthropic infra from `domain`; make OAuth a client-construction concern

**Problem.** `domain/types.go:210-213` holds `BillingHeader` (a Claude Code version-spoof string) and `ClaudeCodeBetas` in the package whose header says "provider-agnostic." `ChatParams.OAuthBilling` (`types.go:182`) threads an auth-mode fact through four packages (`main → agent.WithOAuth → runner → ChatParams → llm`) when it's known once, at wiring time.

**What if some future provider needs a per-request billing flag?** YAGNI — reintroduce a port-level concept when a second provider exists. Auth mode is a construction-time fact today.

**Steps:**

1. RED: `llm` test asserting `NewAnthropicClient(..., llm.WithOAuthBilling())` applies header/betas/thinking on every request (adapt existing httptest assertions).
2. GREEN: move both constants into `internal/llm`; add the adapter option; delete `ChatParams.OAuthBilling` and shrink/remove `agent.WithOAuth` (keep only the max-tokens knob if still needed — consider renaming to `WithMaxTokens`).
3. Note: this simplifies 2.1's factory — children share the client, so billing propagates structurally.
4. Commit: `refactor(domain)!: move OAuth billing into llm adapter construction` (breaking for `ChatParams` shape — internal only).

### 3.3 Unify `tools.ToolDef` with `domain.ToolDef`

**Problem.** Field-for-field duplicate structs bridged by a copy loop (`runner.go:276-284`). No cycle prevents `tools` importing `domain` (domain has zero imports).

**Steps:** RED: compile-driven — change `tools` registration signature to `domain.ToolDef`, delete copy loop, fix compile, keep behavior tests green. Commit: `refactor(tools): use domain.ToolDef directly, delete conversion loop`. (Do together with or right after 2.2 — same code region.)

### 3.4 Move `RunLoginCLI` to `cmd/aql`; make `ResolveAPIKey` pure-core

**Problem.** `RunLoginCLI` (`internal/auth/resolve.go:56-89`) is CLI presentation (stdout printing, flag parsing, `os.Getwd()` — violating the repo's own "thread workDir" rule) inside a library package. `ResolveAPIKey` hardwires `os.Getenv`/`os.UserHomeDir` and swallows `LoadTokens` errors twice (`tokens, _ :=`) — corrupt token file is indistinguishable from absent.

**Steps:** RED: pure test — `resolveAPIKey(tokens, envKey, loadErr)` decision table including the corrupt-file case (assert it surfaces, not swallows) → GREEN: extract pure decision + thin I/O wrapper; move `RunLoginCLI` into `cmd/aql` (it's a subcommand shell). Update `doc/cli/`. Commits: `refactor(auth): extract pure credential resolution decision`, `refactor(cmd): move login CLI shell out of auth package`.

### 3.5 Import-boundary guard test

**Why.** Three separate audits found boundary drift (SDK into agent's closure, tui→diff undocumented, display types in domain). Discipline didn't hold; encode the rules.

**Steps:** RED: a test (e.g. `internal/arch_test.go` using `go/build` or `golang.org/x/tools/go/packages`) asserting: only `llm` imports `anthropic-sdk-go`; `domain` imports stdlib only; `tui` never imports `agent`; `agent` never imports `tui`/`llm`. It should FAIL today (SDK via models) and pass after 3.1. Write it during 3.1 as that step's red test, then keep it forever. Commit: `test: add import-boundary guard`.

### 3.6 Decide OAuth refresh: wire it or delete it

**Problem.** `NeedsRefresh`/`RefreshAccessToken` (`internal/auth/oauth.go:96,229`) and `llm.WithBearerToken` have zero production callers. Sessions silently degrade to `ANTHROPIC_API_KEY` when tokens expire at startup; mid-session expiry has no path at all. Half-built is the worst state: dead weight plus false confidence.

**What if we wire it?** Right shape: a `TokenSource` port (à la `oauth2.TokenSource`) consulted by the llm adapter per request. That's a feature, not a refactor. **What if we delete it?** Honest YAGNI; re-add when refresh is prioritized.
**Decision for the implementing agent:** ask the user. Default if unreachable: **delete** (per YAGNI), keep the memory of the design in an ADR (`doc/adr/`), and add a visible user-facing warning on expired-token fallback (currently only a `slog.Warn`). Commit: `refactor(auth): remove unwired token refresh (YAGNI)` + `docs(adr): record token-refresh design for future wiring`.

---

## Phase 4 — Decompose the TUI God object

Work top-down; each step is independently commitable. The existing sub-state structs (`streamState`, `paletteState`, `diffState`, `modelPickerState`, `transcriptSearchState`, `taskState`) are the seams — the extraction is half-laid already.

### 4.1 Command registry (smallest, do first)

**Problem.** `executeCommand` (`app.go:381-471`) is a 90-line string switch; `Command.Action` is dead; `/model` duplicates `openModelPicker()`; `/exit` aliases handled in two places; adding a command touches two files by documented convention.

**Steps:** RED: test that every `SlashCommands()` entry has a handler and dispatch goes through the registry → GREEN: `Command.Run func(*Model) (tea.Model, tea.Cmd)`; `executeCommand` becomes a lookup; delete `Action`; dedupe `/model` and exit aliases. Commit: `refactor(tui): replace command switch with handler registry`.

### 4.2 Single tool-display descriptor

**Problem.** Per-tool knowledge fragments across five switches/maps (`toolDisplayNames`, `toolInputExtractors`, `FormatToolSummary`, `extractPathFromInput`, `isTaskTool`) keyed by strings owned by another package. Already produced a visible bug: grouped bash renders as "Bashing 3 files...".

**Steps:** RED: (a) test asserting the descriptor map covers exactly `tools` definition names (mirrors `DispatchesAllKnownTools` — closes the cross-package string gap); (b) failing test for the "Bashing 3 files" bug (correct group noun). → GREEN: one `toolDisplay` struct per tool (DisplayName, HeaderArg, Path, Summary, GroupNoun, Silent); the five switches collapse into map lookups. Commits: `test(tui): assert tool display coverage matches tool registry`, `fix(tui): correct grouped non-file tool rendering`, `refactor(tui): unify per-tool display knowledge in one descriptor`.

### 4.3 Purify `View()`: extract layout, evict I/O

**Problem.** `View()` (`app.go:474-586`, 112 lines) mixes overlay short-circuit, viewport math, scroll clamping, and calls `user.Current()` **twice per frame** (`app.go:547` + `welcomeData()`). Scroll clamping is trapped in render (`scrollUp` can't clamp), which is also why `scrollToTranscriptMatch` is a stub.

**Steps:**

1. RED: test `NewModel` resolves user/home once; test that `View` output is byte-identical for identical state without touching `user.Current` (inject the value). GREEN: resolve in `NewModel`/inject from main.
2. RED: pure `layoutChat(entries, width, height, offset) (lines, clampedOffset)` tests: clamping, windowing, padding edge cases (empty, exact-fit, overflow). GREEN: extract; `scrollUp` clamps properly; `View` shrinks to composition of named section renderers (each ≤30 lines).
3. RED: `scrollToTranscriptMatch` real behavior test (jump to match line) — fails against the stub. GREEN: implement on top of `layoutChat`.
4. Commits: `refactor(tui): resolve user info once at construction`, `refactor(tui): extract pure chat layout from View`, `feat(tui): scroll transcript search to matched line`.

### 4.4 Component extraction + focus routing

**Problem.** All behavior for six overlays lives as `Model` methods; four modal surfaces intercept keys at four different layers with implicit precedence; ~25 "(for testing)" accessors + `HandleToolCallExported` exist because nothing is independently constructible.

**What if we adopt full Bubble Tea child-component orthodoxy in one pass?** Too big a bang. **Decision: one component per commit**, easiest first (diff overlay → model picker → palette → transcript search → task panel). Each gets `Update(msg) (component, tea.Cmd)` + `View() string`, direct unit tests, and its `Model` accessors deleted. After two components exist, introduce `activeMode()` focus resolution at the top of `Update` — one explicit precedence list — and route keys to exactly one component.

**Steps per component (repeatable recipe):** RED: port the component's existing behavior tests to construct the component directly (they fail — no component yet) → GREEN: move state struct + its `Model` methods into the component → REFACTOR: delete the corresponding "(for testing)" accessors → commit `refactor(tui): extract <name> component`.

### 4.5 Selection/highlight: stop screen-scraping own output

**Problem.** Mouse selection renders the full frame from inside `Update` (`computeViewLines` calls `m.View()`), regex-strips ANSI, then `highlightLineRange` (`styles.go:51-141`) re-parses escape sequences byte-by-byte to splice highlights — Update depends on View, and the code parses its own output because no intermediate line representation exists.

**Dependency:** do after 4.3 (needs the render-to-lines stage). **Steps:** RED: tests for line-buffer selection extraction + highlight-as-span-operation → GREEN: `View` composes `[]string` lines stored on the model as the single source for selection, highlighting, and viewport slicing; delete the ANSI state machine. Commit: `refactor(tui): select and highlight on line buffer instead of rendered ANSI`.

### 4.6 Delete the dead render pipeline

**Problem.** `RenderChatEntry`, `agent_panel.go`'s `RenderAgentPanel`/`RenderToolBlock`, `RenderPromptAreaStreaming`, `RenderSpinner`, empty `header.go` — zero production callers (grep-verified); only their own tests keep them alive. A second, diverged rendering of the same data.

**Steps:** verify no callers (grep again at execution time — code may have moved), delete code + tests. Keep `RenderAgentHeader`/`RenderUserMessage` (used). Commit: `refactor(tui): delete unused legacy render pipeline`.

### 4.7 Small TUI fixes (batch as individual commits)

- `tsSearch.query[:len-1]` and `picker.input[:len-1]` byte-slice backspace corrupts multibyte input — RED: test with `"héllo"` → GREEN: reuse the existing rune-safe `InputBuffer`. `fix(tui): rune-safe backspace in search and picker inputs`.
- `handleTaskToolResult` returns `true` on unmarshal failure and callers ignore the return — fix semantics or drop the return. `fix(tui): honest handled semantics in task tool result`.
- `RenderMarkdown` constructs a glamour renderer per call in the hottest path — **measure first** (project rule); if confirmed, width-keyed cache. `perf(tui): cache glamour renderer by width` (only with numbers in the commit body).

### 4.8 tui→diff coupling: move display types, bless or break the edge

**Problem.** `tui` imports concrete `internal/diff` types (`types.go:30`, `diff.go:24`) — an edge absent from the documented architecture graph. Separately, `domain` holds `ToolCall`/`ToolStatus` which its own header calls display concepts.

**Decision:** move `DiffFile`/`DiffStats`/`DiffLine` into `domain` (cheapest, one source of truth) **or** update `doc/architecture` to bless tui→diff — implementing agent picks based on whether diff types are needed outside tui (check `cmd/aql` usage); don't leave code and docs disagreeing. Move `ToolCall`/`ToolStatus` from `domain` into `tui` (adapter constructs the TUI message directly — `tui` already imports `domain`, no cycle). Commits: `refactor(domain): move display-only tool call types into tui`, `docs(architecture): reconcile diff dependency edge`.

---

## Phase 5 — Remaining runner/tools hygiene

- **`Run()` closure ~60 lines** (`runner.go:82-144`) mixing loop control, retry, compaction policy, token accounting: extract named `runConversationLoop` + per-iteration step fn (concurrency rule: named goroutine bodies). RED via existing behavior tests staying green + new unit test on the extracted step. `refactor(agent): extract conversation loop from Run goroutine`.
- **Fabricated token event:** `maybeAutoCompact` emits `TokenUsageEvent{InputTokens: len(summary)/4}` — an estimate faked as a precise count (violates the field's documented meaning). RED: test asserting no fake usage event / a dedicated field → GREEN: add `Estimated bool` or drop the event. `fix(agent): stop emitting estimated tokens as precise usage`.
- **Auto-compact pipeline end-to-end test** (currently 22% covered, positive path never runs): fake client returning >threshold usage → assert compaction, `HistoryReplaceMsg`, `ReplaceHistory` applied, TUI message order. Combine with 1.4. `test(agent): cover auto-compact positive path end-to-end`.
- **`AutoCompactThreshold = 160_000` global** assumes 200k window for all models; `domain.ModelInfo.MaxInputTokens` exists and is unused — derive threshold from model info. RED first. `refactor(agent): derive compact threshold from model window`.
- **Output truncation policy** is inconsistent (grep 10k inline magic, web 50k, glob 500, `read_file`/`list_directory` unbounded — one huge file blows the context window): central truncation helper with per-tool limits; RED: `read_file` on an oversized file returns truncated-with-marker. `fix(tools): bound read_file and list_directory output`.
- **`web_search` provider seam** (DDG URL + CSS classes hardcoded): tolerable YAGNI today; only add `SearchProvider` port if/when a second provider or a DDG breakage forces it. Note in ADR, do nothing now.
- **Sub-agent observability** (children's events discarded; parallel children collide on `name-sub-1` names): fix the name collision now (RED: two spawns → distinct names; add a counter). Event propagation is a feature decision — record in `doc/adr/`. `fix(agent): unique sub-agent names for parallel spawns`.
- **Testable `run()`:** thread `args []string`, `workDir`, output writer, HTTP client into `run()` (or extract `wire(cfg)` builder); also stops `aql.log` landing in the tester's cwd. RED: smoke test constructing the wiring without side effects. `refactor(cmd): make run() honor its testable-entrypoint purpose`.
- **Login callback races:** `login.go:112` sleeps 50ms hoping the server started; `findAvailablePort` closes then re-binds (TOCTOU). Fix: `net.Listen` once, derive port from listener, `srv.Serve(listener)`. RED: test that callback is servable immediately after the flow reports ready. `fix(auth): remove callback server startup race`.

---

## Phase 6 — E2E and test hygiene

- **Assertion-free e2e tests:** `TestE2E_TypeAndSlashHelp`, `TestE2E_CtrlCExit` (never asserts exit), `TestE2E_EditFileShowsDiff` (ends in `t.Logf` — permanently green spec of an unimplemented check). Convert each: `WaitForFunc` on expected screen content / process-exit assertion. If the diff scenario can't pass yet, make it `t.Skip` with a reason — never silently green. `test(e2e): add assertions to screenshot-only scenarios`.
- **Blind sleeps:** `scenario_test.go:131,136,141` and `recorder_test.go:213,220` — replace with `WaitForFunc`; the recorder needs a real flush/done signal (expose it on `Recorder`, RED first). `fix(e2e): replace recorder sleeps with completion signal`.
- **mtime sleep** in `agent_test.go:84`: use `os.Chtimes` (10ms can under-shoot fs mtime granularity — flake in both directions). `test(agent): deterministic mtime change via Chtimes`.
- **Timing assertion** in `runner_replay_test.go:239-315`: delete `elapsed < 120ms` (keep the deterministic `maxConcurrent >= 2`). `test(agent): drop wall-clock assertion from parallel tool test`.
- **Untagged PTY tests:** `terminal_test.go`/`recorder_test.go` spawn real PTYs in plain `go test ./...` — move behind the `e2e` tag or `-short` guard. `test(e2e): gate harness self-tests behind e2e tag`.
- **Constant test:** `compact_test.go:142-144` asserts `AutoCompactThreshold == 160_000` — zero behavioral value; delete when Phase 5 derives the threshold.
- **Convention drift:** `internal/agent/errors_test.go` uses internal `package agent` — either convert to `_test` (export a tiny seam) or document it as an exception next to the tui one in `.claude/rules/architecture.md`.
- **`models/startup.go` 0%:** unit-test `LoadOrDefault`/`ModelToTier`/`ProbeAndUpdate` with the fake prober from 3.1.

---

## Dependency graph (what blocks what)

```
1.1 fake ChatClient ──► 2.1, 2.3, 2.4, 5.* (all agent TDD)
1.4 ForwardWithHistory test ──► safe to touch stream/history anywhere
2.2 executor defs ──► 3.3 (same region — do adjacent)
3.1 models split ──► 3.5 boundary test passes ──► 3.2 rides the same adapter
3.2 OAuth to adapter ──► simplifies what 2.1's factory carries (2.1 first is still correct)
4.3 layoutChat ──► 4.5 selection rewrite, scroll-to-match
4.1, 4.2, 4.6, 4.7 — independent, any order
Phase 6 — independent, can interleave anywhere
```

## Risks / open questions for the implementing agent

1. **3.6 (OAuth refresh)** — needs user decision: wire a TokenSource port or delete. Default: delete + ADR.
2. **4.8 (diff types)** — pick move-to-domain vs bless-the-edge after checking real usage.
3. **2.3 race test** — if `-race` doesn't trip deterministically, document how the red state was demonstrated rather than shipping a flaky test.
4. **Changelog discipline:** every phase completion gets a `doc/changelog/2026/MM/DD-HHMM-*.md` entry; boundary changes get `doc/adr/` records (models split, OAuth-to-adapter, display-type moves).
5. **What NOT to touch:** `internal/domain`'s port design, `internal/llm`'s conversion layer, `internal/stream`'s adapter role, the `BuildTranscriptBlocks` transcript pipeline, `InputBuffer`/`History`/`Selection` functional cores, the record/replay e2e design. These audited clean.
