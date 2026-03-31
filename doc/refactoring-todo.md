# Refactoring TODO

Identified 2026-03-31. Work through top-to-bottom by priority.

---

## High Priority

- [x] **1. Split Model god object** — partially done (handlers extracted, Model struct still large)
  - [ ] Extract `CommandPaletteState`, `StreamingState`, `SelectionState`
  - [ ] Extract rendering into separate modules
  - [x] Split `View()` delegates are clean; `executeCommand()` is manageable

- [x] **2. Inject Anthropic client** — `internal/models/probe.go`
  - 6 public functions collapsed to 2 (`FetchModels`, `ProbeUsableModels`) + `ClientConfig` struct

- [x] **3. Split large TUI handlers** — `internal/tui/handlers.go`
  - `handleMsg()` — thin dispatcher, each case extracted to own method
  - `handleKey()` — transcript mode, esc, tab, up/down extracted
  - `handleSubmit()` — answer, bash, normal submit extracted

- [x] **4. Extract agent runner concerns** — `internal/agent/runner.go`
  - `Run()` is now ~50 line orchestrator
  - Extracted `streamWithRetry()`, `consumeStream()`, `executeTools()`, `maybeAutoCompact()`

- [x] **5. Clean up main bootstrap** — `cmd/aql/main.go`
  - Extracted `setupLogging()`, `configureTUI()`, `startBackgroundModelProbe()`

---

## Medium Priority

- [ ] **6. Wrap SDK types in domain interfaces** — `internal/agent/`
  - `anthropic.MessageParam`, `anthropic.Model`, `anthropic.ContentBlockParamUnion` used directly
  - Create domain types and adapter wrappers
  - Deferred: large cross-cutting change, needs careful planning

- [x] **7. Split Login flow** — `internal/auth/login.go`
  - Extracted `callbackServer` struct, `startCallbackServer()`, `openAuthURL()`, `exchangeAndCreateKey()`
  - `Login()` is now a clean 25-line orchestrator

- [x] **8. Centralize tool input parsing** — reviewed, already well-organized
  - `extractField`, `quoteField`, `extractPathFromInput` are cohesive with rendering in transcript.go
  - No extraction needed

- [x] **9. Centralize OAuth/billing config**
  - Constants moved to `domain.BillingHeader` and `domain.ClaudeCodeBetas`
  - `agent/runner.go` uses domain constants directly; `models/probe.go` re-exports for compat

- [x] **10. Extract callback closures** — reviewed, already clean
  - Closures in `configureTUI()` are inherently tied to captured state
  - A struct would add indirection without benefit

---

## Low Priority

- [x] **11. Consolidate TUI styles** — already mostly in `styles.go`
  - Moved 2 inline styles from `prompt.go` to `PromptLineStyle`/`PromptBadgeStyle`
- [x] **12. Extract SystemPromptBuilder** — reviewed, not needed
  - `BuildSystemPrompt()` is 24 lines, reads cleanly, YAGNI applies
- [x] **13. Split InputBuffer** — reviewed, not needed
  - 124 lines, cursor and text manipulation are inherently coupled
- [x] **14. Move enrichAPIError** — done
  - Moved `isRetryableError` and `enrichAPIError` to `internal/agent/errors.go`
