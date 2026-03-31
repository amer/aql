# Next Sprint: Quick Wins

Five size-S features that give the agent essential context and reliability.

## 1. Git status + env info + date injection

- [x] Run `git status --short` and `git branch --show-current` at startup
- [x] Collect env: `runtime.GOOS`, `os.Getenv("SHELL")`, CWD, Go version
- [x] Inject `time.Now().Format("2006-01-02")` as current date
- [x] Add `EnvironmentInfo() string` to `internal/agent`
- [x] Call from `BuildSystemPrompt()` — auto-injected on agent creation
- [x] Test: assert system prompt contains date, platform, model

## 2. Precise token counting

- [x] Parse `usage.input_tokens` and `usage.output_tokens` from `MessageDeltaEvent` in `runner.go`
- [x] Emit `TokenUsageEvent` via `StreamEvent` channel
- [x] Add `TokenUsageMsg` to TUI, sets `tokenCount = input + output`
- [x] Wire in `main.go` stream consumer to forward `TokenUsage` to TUI
- [x] Test: `TokenUsageMsg` sets precise counts, updates on each message

## 3. Dynamic tool descriptions in system prompt

- [x] Add `ToolDescriptionsPrompt() string` that iterates `ToolDefinitions()`
- [x] Format each tool as `- tool_name: description`
- [x] Remove hardcoded tool list from `main.go` system prompt
- [x] Injected automatically via `BuildSystemPrompt()` — new tools auto-appear
- [x] Test: assert all tool names present, assert matches `ToolDefinitions()`

## 4. CLAUDE.md hot-reload

- [x] Added `RefreshClaudeMD()` method with mtime-based cache on `Agent`
- [x] Called at the start of `buildMessageParams()` — every API call checks for changes
- [x] Initial mtime stored on agent creation; only re-reads when file changes
- [x] Test: write CLAUDE.md, refresh, assert content; update file, refresh again, assert new content

## 5. Auto-compaction

- [x] After each API response in `runner.go`, check `inputTokens` vs `AutoCompactThreshold`
- [x] Threshold: 160,000 tokens (80% of 200k context window)
- [x] If exceeded on end_turn, call `CompactHistory()` automatically in the runner
- [x] Emit updated `TokenUsageEvent` after compaction so TUI reflects reduced count
- [x] Test: `AutoCompactThreshold` constant is 160k
