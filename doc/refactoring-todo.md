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

- [ ] **7. Split Login flow** — `internal/auth/login.go` `Login()` (98 lines)
  - Extract HTTP callback server, browser opening, token exchange

- [ ] **8. Centralize tool input parsing** — `internal/tui/transcript.go`
  - `extractField`, `quoteField`, `extractPathFromInput` mixed with rendering
  - Move to dedicated module

- [ ] **9. Centralize OAuth/billing config**
  - Scattered across `internal/models/probe.go` and `internal/agent/runner.go`
  - Create `OAuthConfig` struct

- [ ] **10. Extract callback closures** — `cmd/aql/main.go` (lines 72-155)
  - Create `TUICallbacks` struct with named methods

---

## Low Priority

- [ ] **11. Consolidate TUI styles** — scattered across `internal/tui/` files into `styles.go`
- [ ] **12. Extract SystemPromptBuilder** — `internal/agent/agent.go` `BuildSystemPrompt()` mixes 5 concerns
- [ ] **13. Split InputBuffer** — `internal/tui/input.go` mixes text manipulation and cursor logic
- [ ] **14. Move enrichAPIError** — `internal/agent/runner.go` to its own file for reuse
