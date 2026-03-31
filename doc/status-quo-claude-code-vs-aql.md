# Status Quo: Claude Code vs AQL

> Snapshot as of 2026-03-31. Claude Code is the reference implementation (TypeScript). AQL is a Go reimplementation.

---

## At a Glance

| Dimension         | Claude Code                        | AQL                              |
| ----------------- | ---------------------------------- | -------------------------------- |
| **Language**      | TypeScript (React 19 + Node.js)    | Go (Bubble Tea)                  |
| **Codebase size** | ~1,884 files, ~25 MB source        | ~120 files, ~17K LOC             |
| **Test code**     | Not in snapshot                    | ~50 files, ~9.8K LOC             |
| **Architecture**  | Layered monolith, React reconciler | Ports & adapters, event channels |
| **UI framework**  | Custom Ink (React → ANSI terminal) | Bubble Tea (Elm architecture)    |
| **Layout engine** | Yoga (WASM flexbox)                | Bubble Tea + Lip Gloss           |
| **Maturity**      | Production (shipping to millions)  | Early development                |

---

## Architecture Comparison

### Agent Loop

| Aspect                 | Claude Code                                                                  | AQL                                                                               |
| ---------------------- | ---------------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| **Core file**          | `query.ts` (~68 KB)                                                          | `internal/agent/runner.go` (~190 lines)                                           |
| **Pattern**            | Async generator (`async function*`) yielding messages                        | Goroutine emitting to buffered `chan domain.StreamEvent`                          |
| **Turns**              | Configurable max turns + budget limit                                        | Max 25 tool iterations                                                            |
| **Tool execution**     | `StreamingToolExecutor` — parallel for concurrent-safe tools                 | Sequential — one tool at a time                                                   |
| **Error recovery**     | Max-output escalation, reactive compaction, streaming fallback, model switch | 2-attempt retry with exponential backoff (500/529)                                |
| **History management** | Mutable `Message[]` in `QueryEngine`, normalized per API call                | Race-free: Run() works on snapshot, emits mutation events to single-writer caller |
| **Compaction**         | Auto (token threshold), snip, micro, context-collapse                        | Manual (`/compact` command); auto-detection exists but not wired                  |
| **Streaming**          | Raw stream processing (avoids O(n²) JSON parsing)                            | Anthropic SDK streaming with callback-based text deltas                           |

### Tool System

| Aspect            | Claude Code                                                          | AQL                                                             |
| ----------------- | -------------------------------------------------------------------- | --------------------------------------------------------------- |
| **Count**         | 40+ tools                                                            | 14 tools                                                        |
| **Interface**     | Generic `Tool<Input, Output, Progress>` with ~30 methods             | `func(ctx, input) (string, error)` registered in map            |
| **Schema**        | Zod schemas with lazy construction, deferred loading                 | Static JSON schema definitions in `defs.go`                     |
| **Permissions**   | Per-tool `checkPermissions()` → allow/ask/deny                       | None — all tools execute without approval                       |
| **Rendering**     | Each tool has React rendering methods (`renderToolUseMessage`, etc.) | TUI renders via `toolDisplayNames` + `toolInputExtractors` maps |
| **Concurrency**   | Declared per-tool (`isConcurrencySafe`)                              | All sequential                                                  |
| **Progress**      | `onProgress` callback with typed `ToolProgress<P>`                   | No progress reporting                                           |
| **Result budget** | Large results persisted to disk, stub sent to Claude                 | Full results always in context                                  |

**Tool inventory:**

| Tool           | Claude Code                                            | AQL                                      |
| -------------- | ------------------------------------------------------ | ---------------------------------------- |
| File read      | `FileReadTool` (images, PDF, notebooks, token-limited) | `read_file` (text only)                  |
| File write     | `FileWriteTool` (stale check, LSP notify)              | `write_file` (basic)                     |
| File edit      | `FileEditTool` (stale detection, patch gen, LSP, diff) | `edit` (find/replace with `replace_all`) |
| Shell          | `BashTool` (auto-background, git tracking, security)   | `bash` (basic `sh -c`)                   |
| Glob           | `GlobTool` (100-file limit, concurrency-safe)          | `glob` (Go-native, newest-first)         |
| Grep           | `GrepTool` (ripgrep, pagination, context lines, modes) | `grep` (wraps grep binary)               |
| Web fetch      | `WebFetchTool` (preapproved hosts, deferred)           | `web_fetch` (basic)                      |
| Web search     | `WebSearchTool` (server-side, max 8/call)              | `web_search` (basic)                     |
| Ask user       | `AskUserQuestionTool`                                  | `ask_user`                               |
| Agent spawn    | `AgentTool` (worktree, model override, team colors)    | `agent` (depth limit of 3)               |
| Task create    | `TaskCreateTool` (hooks, auto-expand)                  | `task_create`                            |
| Task update    | `TaskUpdateTool`                                       | `task_update`                            |
| Task list      | `TaskListTool`                                         | `task_list`                              |
| Notebook edit  | `NotebookEditTool`                                     | `notebook_edit`                          |
| Task get       | `TaskGetTool`                                          | —                                        |
| Task stop      | `TaskStopTool`                                         | —                                        |
| Task output    | `TaskOutputTool`                                       | —                                        |
| List directory | — (uses Glob/Bash)                                     | `list_directory`                         |
| Plan mode      | `EnterPlanModeTool` / `ExitPlanModeTool`               | —                                        |
| Worktree       | `EnterWorktreeTool` / `ExitWorktreeTool`               | —                                        |
| MCP proxy      | `MCPTool`                                              | —                                        |
| LSP            | `LSPTool`                                              | —                                        |
| Skill invoke   | `SkillTool`                                            | —                                        |
| Tool search    | `ToolSearchTool`                                       | —                                        |
| Send message   | `SendMessageTool` (multi-agent)                        | —                                        |
| Config         | `ConfigTool`                                           | —                                        |
| Sleep          | `SleepTool`                                            | —                                        |
| REPL           | `REPLTool`                                             | —                                        |
| Cron           | `ScheduleCronTool`                                     | —                                        |
| Todo           | `TodoWriteTool`                                        | —                                        |
| MCP resources  | `ListMcpResourcesTool` / `ReadMcpResourceTool`         | —                                        |
| MCP auth       | `McpAuthTool`                                          | —                                        |
| Remote trigger | `RemoteTriggerTool`                                    | —                                        |
| Team manage    | `TeamCreateTool` / `TeamDeleteTool`                    | —                                        |

### Terminal UI

| Aspect            | Claude Code                                                                          | AQL                                          |
| ----------------- | ------------------------------------------------------------------------------------ | -------------------------------------------- |
| **Framework**     | Custom Ink (React reconciler → Yoga → ANSI)                                          | Bubble Tea (Elm-style Update/View)           |
| **Rendering**     | 60fps double-buffered, cell pooling, frame diffing, hardware scroll                  | Bubble Tea's built-in renderer               |
| **Layout**        | Yoga WASM flexbox                                                                    | Manual width/height calculation              |
| **Components**    | 147 component dirs (messages, permissions, design-system, diff, tasks)               | Single `app.go` Model (~1,600 LOC) + helpers |
| **Input**         | PromptInput (~8K lines): buffer, cursor, selection, history, suggestions, modes, vim | Multi-line input with history navigation     |
| **Messages**      | 36+ type-specific renderers (text, tool use, thinking, images, diffs)                | Glamour markdown + tool call/result blocks   |
| **Scrolling**     | Virtual scroll (`useVirtualScroll`) for thousands of messages                        | Viewport-based scroll                        |
| **Keyboard**      | 18 contexts, 50+ actions, chord support, customizable keybindings.json               | Basic keybindings (Ctrl+C, Ctrl+O, arrows)   |
| **Focus**         | FocusManager with capture/bubble event delegation                                    | Bubble Tea focus zones                       |
| **Fullscreen**    | AlternateScreen for modals, diffs, settings                                          | Model picker overlay                         |
| **Search**        | Transcript search with match highlighting                                            | Ctrl+O search in transcript                  |
| **Diff viewer**   | Fullscreen diff dialog (side-by-side / unified)                                      | —                                            |
| **Permission UI** | 32+ specialized permission dialogs                                                   | —                                            |
| **Voice**         | Push-to-talk with STT streaming                                                      | —                                            |
| **Vim mode**      | Full vim keybindings for input                                                       | —                                            |

### Permission System

| Aspect               | Claude Code                                                           | AQL  |
| -------------------- | --------------------------------------------------------------------- | ---- |
| **Modes**            | 6 modes: default, acceptEdits, bypassPermissions, plan, auto, dontAsk | None |
| **Rules**            | Per-tool rules from settings, CLI flags, hooks, classifiers           | —    |
| **Classifier**       | 2-stage yolo classifier (fast XML + thinking model)                   | —    |
| **Hook integration** | PreToolUse/PostToolUse hooks can modify permissions                   | —    |
| **UI**               | Type-specific dialogs (Bash, FileEdit, FileWrite, etc.)               | —    |

### Hook System

| Aspect           | Claude Code                                                                            | AQL |
| ---------------- | -------------------------------------------------------------------------------------- | --- |
| **Events**       | 10+ events (SessionStart, PreToolUse, PostToolUse, UserPromptSubmit, FileChanged, ...) | —   |
| **Hook types**   | Shell commands, HTTP POST, LLM prompt, agent verification                              | —   |
| **Conditions**   | Pattern matching (e.g., `Bash(git *)`, `FileEdit(/src/**/*.ts)`)                       | —   |
| **Capabilities** | Modify input/output, inject messages, change permissions, block execution              | —   |

### Command System

| Aspect       | Claude Code                                                                                                                                                        | AQL                                                        |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------- |
| **Count**    | 104 command directories, 150+ commands                                                                                                                             | 6 commands                                                 |
| **Types**    | PromptCommand, LocalCommand, LocalJSXCommand                                                                                                                       | Slash commands handled in TUI                              |
| **Commands** | `/commit`, `/review`, `/help`, `/clear`, `/model`, `/diff`, `/resume`, `/plan`, `/memory`, `/mcp`, `/config`, `/login`, `/logout`, `/stats`, `/vim`, `/voice`, ... | `/clear`, `/compact`, `/model`, `/tasks`, `/help`, `!bash` |
| **Loading**  | Lazy-loaded modules                                                                                                                                                | Inline switch in TUI                                       |

### System Prompt

| Aspect                | Claude Code                                                                                         | AQL                                      |
| --------------------- | --------------------------------------------------------------------------------------------------- | ---------------------------------------- |
| **Assembly**          | Multi-part: role, tools, environment, git status, CLAUDE.md, memory, date, model info, output style | Static: role + system prompt from config |
| **Context injection** | Git status (branch, commits, dirty), OS/platform/shell, date, model capabilities                    | —                                        |
| **CLAUDE.md**         | Hot-reload on file change, directory walking for nested files                                       | Loaded once at startup, no reload        |
| **Memory**            | Auto-memory system with 4-type taxonomy, semantic search, MEMORY.md index                           | —                                        |
| **Dynamic tools**     | Tool descriptions injected into prompt, deferred tools hidden until searched                        | Static tool list                         |
| **Output styles**     | Configurable (default, explanatory, learning, custom)                                               | —                                        |

### Session Management

| Aspect                  | Claude Code                                                               | AQL                |
| ----------------------- | ------------------------------------------------------------------------- | ------------------ |
| **Persistence**         | Full session serialization to `~/.claude/projects/`                       | — (in-memory only) |
| **Resume**              | `--continue`, `--resume [id]`, session picker                             | —                  |
| **History**             | `~/.claude/history.jsonl` with pasted content dedup                       | —                  |
| **Metadata**            | Session ID, timestamps, git branch, model, cost, PR link, tags, summaries | —                  |
| **Content replacement** | Large tool results persisted to disk, stubs in conversation               | —                  |

### Authentication

| Aspect             | Claude Code                                                           | AQL                                                |
| ------------------ | --------------------------------------------------------------------- | -------------------------------------------------- |
| **Methods**        | OAuth (Claude.ai), API key, AWS Bedrock, Google Vertex, Azure Foundry | OAuth PKCE (Claude.ai + Console), API key fallback |
| **Token storage**  | macOS Keychain (prefetched at startup)                                | Disk-based token files                             |
| **Multi-provider** | Yes (5 providers with different auth flows)                           | No (Anthropic direct only)                         |
| **Enterprise**     | MDM policies, org validation, force-login                             | —                                                  |

### Extensibility

| Aspect            | Claude Code                                                                      | AQL |
| ----------------- | -------------------------------------------------------------------------------- | --- |
| **MCP**           | Full MCP client (stdio/SSE/HTTP/WebSocket), 25 files, config from 4 sources      | —   |
| **Plugins**       | Plugin system with marketplace, versioning, hot-reload                           | —   |
| **Skills**        | Markdown prompt templates in `.claude/skills/`, bundled skills, change detection | —   |
| **Output styles** | Customizable via `.claude/output-styles/` markdown files                         | —   |
| **Custom agents** | Agent definitions loaded from config, model/tool/prompt overrides                | —   |
| **Hooks**         | User-defined shell/HTTP/prompt/agent hooks on 10+ events                         | —   |

### Remote / Multi-Agent

| Aspect                 | Claude Code                                                                | AQL                                |
| ---------------------- | -------------------------------------------------------------------------- | ---------------------------------- |
| **Bridge mode**        | Full Remote Control: CLI ↔ environments API ↔ Claude.ai sessions (~900 KB) | —                                  |
| **Multi-agent**        | Team mode with teammate spawning, color assignment, concurrent sessions    | Basic sub-agent with depth=3 limit |
| **Background agents**  | Ctrl+B backgrounds, async notifications, task panel                        | —                                  |
| **Worktree isolation** | Git worktrees for concurrent agent execution                               | —                                  |
| **Coordinator**        | Multi-agent orchestration with controlled tool sets                        | —                                  |

### Configuration

| Aspect             | Claude Code                                                         | AQL |
| ------------------ | ------------------------------------------------------------------- | --- |
| **Settings files** | `~/.claude/settings.json` (user), `.claude/settings.json` (project) | —   |
| **Keybindings**    | `~/.claude/keybindings.json` with 18 contexts, 50+ actions          | —   |
| **MCP config**     | `.mcp.json`, enterprise managed, Claude.ai cloud                    | —   |
| **Migrations**     | 12 migration scripts for settings/model upgrades                    | —   |
| **MDM**            | macOS/Windows enterprise policy enforcement                         | —   |
| **Feature gates**  | GrowthBook for A/B testing and rollout                              | —   |

### Cost & Observability

| Aspect              | Claude Code                                                         | AQL                                 |
| ------------------- | ------------------------------------------------------------------- | ----------------------------------- |
| **Cost tracking**   | Per-model USD, tokens (input/output/cache), duration, lines changed | Token count displayed in status bar |
| **Analytics**       | Datadog APM, GrowthBook, first-party event logging                  | —                                   |
| **Telemetry**       | OpenTelemetry with deferred initialization                          | —                                   |
| **Profiling**       | Startup profiling checkpoints, yoga/commit timing                   | —                                   |
| **Session metrics** | Persisted per-session for resume cost recovery                      | —                                   |

---

## What AQL Does Well

These are architectural strengths where AQL's design is sound — building blocks that support future features without rework:

1. **Race-free history** — Event-driven single-writer pattern. Claude Code uses mutable arrays with careful sequencing; AQL's channel-based design is more rigorous.

2. **Clean dependency graph** — `domain → agent → stream → tui` with no circular imports. Claude Code has circular-dependency prevention via getter/setter singletons, which is more fragile.

3. **Minimal surface area** — 17K LOC vs 25 MB. Every line is intentional. Adding features is additive, not archaeological.

4. **Test discipline** — ~60% test-to-source ratio, external test packages, table-driven tests, fakes over mocks. Testing the right things at the right level.

5. **Ports & adapters** — `domain.ChatClient` interface cleanly decouples from Anthropic SDK. Adding Bedrock/Vertex is an adapter, not a rewrite.

6. **Tool registration contract** — Three-part system (definition, handler, display) enforced by tests. Adding a tool means implementing three things, not hunting through a codebase.

---

## What AQL Needs Next

Ordered by impact — features that unlock the most capability per effort:

### Tier 1: Safety & Usability (blocks real usage)

| Feature                    | Why                                                            | Size |
| -------------------------- | -------------------------------------------------------------- | ---- |
| **Permission system**      | Tools execute without user approval — unsafe for write/bash    | M    |
| **Session persistence**    | Conversations lost on exit — kills multi-session workflows     | M    |
| **System prompt assembly** | No git/env/date context — Claude operates blind to environment | S    |

### Tier 2: Context & Intelligence (makes Claude smarter)

| Feature                     | Why                                                                | Size |
| --------------------------- | ------------------------------------------------------------------ | ---- |
| **Auto-compaction**         | Long conversations silently degrade — manual `/compact` not enough | S    |
| **CLAUDE.md hot-reload**    | Rules file changes ignored mid-session                             | S    |
| **Content replacement**     | Large tool results exhaust context window                          | S    |
| **Parallel tool execution** | Sequential tools are 2-5x slower for independent operations        | M    |

### Tier 3: Power User Features (differentiators)

| Feature                      | Why                                         | Size                  |
| ---------------------------- | ------------------------------------------- | --------------------- |
| **Resume (`--continue`)**    | Pick up where you left off                  | S (needs persistence) |
| **Diff viewer**              | File edits shown inline without visual diff | M                     |
| **Extended thinking toggle** | No way to see/control Claude's reasoning    | S                     |
| **Background agents**        | Sub-agents block the main loop              | M                     |

### Tier 4: Ecosystem (long-term)

| Feature                 | Why                                   | Size |
| ----------------------- | ------------------------------------- | ---- |
| **MCP support**         | Can't use external tool servers       | L    |
| **Hook system**         | No automation / CI integration points | L    |
| **Skill/plugin system** | No user-defined prompt expansion      | M    |
| **Settings system**     | No persistent user preferences        | M    |

---

## Quantitative Summary

| Metric              | Claude Code                              | AQL         | Ratio |
| ------------------- | ---------------------------------------- | ----------- | ----- |
| Source files        | ~1,884                                   | ~120        | 16:1  |
| Tools               | 40+                                      | 14          | 3:1   |
| Commands            | 150+                                     | 6           | 25:1  |
| UI components       | 147 dirs                                 | 1 main file | —     |
| Hook events         | 10+                                      | 0           | —     |
| MCP transports      | 4 (stdio/SSE/HTTP/WS)                    | 0           | —     |
| Auth providers      | 5                                        | 2           | 2.5:1 |
| Permission modes    | 6                                        | 0           | —     |
| Settings sources    | 6 (CLI, env, project, user, MDM, remote) | 0           | —     |
| Session persistence | Full (resume, history, metadata)         | None        | —     |
| Test files          | Not in snapshot                          | 50          | —     |

AQL has ~6% of Claude Code's surface area but covers the core agent loop, tool system, TUI, and auth — the foundation that everything else builds on.
