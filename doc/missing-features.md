# Missing Features: AQL vs Claude Code

Gap analysis derived from the Claude Code v2.1.88 system prompt and binary.

## Legend

- **Scope**: where the change lives (`agent`, `tui`, `main`, `new-pkg`)
- **Size**: T-shirt estimate (S/M/L/XL)

---

## 1. System Prompt Assembly

Claude Code assembles its system prompt from ~12 dynamic sections. AQL has a hardcoded string in `main.go`.

### 1.1 Git status injection

CC injects repo status (branch, dirty files) into every conversation start as `gitStatus` context.

**How**: In `main.go`, run `git status --short` and `git branch --show-current` before creating the agent. Pass result as a field on `Config` or a dedicated `SystemContext` struct. Append to system prompt in `BuildSystemPrompt()`.
**Scope**: `main`, `agent` | **Size**: S

### 1.2 Environment info section

CC injects platform, shell, OS version, CWD, model ID, knowledge cutoff, and whether the dir is a git repo.

**How**: New `agent.EnvironmentInfo()` function using `runtime.GOOS`, `os.Getenv("SHELL")`, `exec.Command("uname", "-r")`. Call from `main.go` and inject into system prompt.
**Scope**: `agent` | **Size**: S

### 1.3 Dynamic tool descriptions in system prompt

CC's system prompt describes every available tool with usage instructions. AQL's system prompt has a static list that drifts from actual tool definitions.

**How**: Generate the tool description section from `ToolDefinitions()` at startup. Each `ToolDef` already has a description — concatenate them into a prompt section automatically.
**Scope**: `agent` | **Size**: S

### 1.4 Memory injection into system prompt

AQL has `BuildSystemPromptWithMemories()` but never calls it. Memory is scaffolding only.

**How**: In `runner.go`'s `buildMessageParams()`, call `BuildSystemPromptWithMemories()` instead of `BuildSystemPrompt()`. Requires wiring the memory manager's `Query()` to produce a context string.
**Scope**: `agent` | **Size**: M

### 1.5 CLAUDE.md hot-reload

CC re-reads CLAUDE.md files per request. AQL loads once at agent creation and bakes it in.

**How**: Move `CollectClaudeMD()` call from `agent.New()` into `buildMessageParams()` (or cache with mtime check). This ensures edits to CLAUDE.md are picked up without restarting.
**Scope**: `agent` | **Size**: S

### 1.6 Current date injection

CC injects `currentDate` into every system prompt so the model knows today's date.

**How**: Add `time.Now().Format("2006-01-02")` to the system prompt template in `BuildSystemPrompt()`.
**Scope**: `agent` | **Size**: S

---

## 2. Tool Permission System

CC has a multi-mode permission system (accept-edits, plan, full-auto, etc.) that prompts the user before sensitive tool calls.

### 2.1 Tool approval prompt

Before executing write/bash tools, pause and ask the user to approve.

**How**: Add a `Permission` field to `ToolDef` (e.g. `AlwaysAllow`, `RequireApproval`). In `executeTool()`, if approval required, send a new `ToolApprovalMsg` to the TUI via the event channel. Block on a response channel (like `ask_user` already does). TUI shows a y/n prompt.
**Scope**: `agent`, `tui` | **Size**: M

### 2.2 Permission modes

CC has modes like `plan` (read-only), `acceptEdits`, `fullAuto`. Users cycle with Shift+Tab.

**How**: Define a `PermissionMode` enum. Store on the `Model`. Each mode maps tool names to allow/deny/ask. Expose `/permissions` slash command and `Shift+Tab` keybinding to cycle. Feed the mode into the runner so it filters tool execution.
**Scope**: `tui`, `agent` | **Size**: L

---

## 3. Sub-agents

CC's Agent tool spawns child agents with isolated conversations, optional worktree isolation, and background execution.

### 3.1 Basic sub-agent spawning — IMPLEMENTED

An `agent` tool that creates a child `Agent` with its own history, runs a prompt, returns the result.

**Status**: Implemented in `internal/agent/spawner.go` (Spawner, depth-capped, registered as the `agent` tool via `tools.WithAgentSpawner`). Children inherit the parent's agent options (OAuth billing, etc.) via `WithAgentOptions`.

### 3.2 Background agents

CC can run agents asynchronously (`run_in_background: true`) and notify when done.

**How**: Wrap sub-agent execution in a goroutine. Return an interim result with an agent ID. Add a `BackgroundAgentDoneMsg` to the TUI. Store running agents in a map on the orchestrator. Add `/tasks` or `/agents` command to check status.
**Scope**: `agent`, `tui`, `orchestrator` | **Size**: XL

### 3.3 Worktree isolation

CC creates temporary git worktrees so sub-agents work on isolated copies.

**How**: Before spawning a sub-agent, run `git worktree add <tmp-path> -b <branch>`. Set the child agent's `dir` to the worktree path. Clean up on completion if no changes.
**Scope**: `agent` | **Size**: M

---

## 4. Task Tracking

CC has TaskCreate/TaskUpdate/TaskList tools that let the agent break work into visible steps.

**How**: Add `task_create`, `task_update`, `task_list` tool definitions. Store tasks in a `[]Task` slice on the agent (or a shared task store). Each task has ID, description, status (pending/in_progress/completed). Surface in TUI via `/tasks` command or a persistent sidebar. Agent sees task list in tool results.
**Scope**: `agent`, `tui` | **Size**: M

---

## 5. Plan Mode

CC has an `EnterPlanMode`/`ExitPlanMode` tool that restricts the agent to read-only tools while designing an approach.

**How**: Add a `planMode bool` on the agent. When true, `executeTool()` rejects write/bash tools and returns an error message. Add `/plan` slash command to toggle. Inject "you are in plan mode" into system prompt when active.
**Scope**: `agent`, `tui` | **Size**: M

---

## 6. Session Persistence

CC persists every conversation to disk and supports `/resume` to pick up previous sessions.

### 6.1 Session save

**How**: After each message round-trip, append the conversation turn to a JSONL file in `~/.aql/sessions/<id>.jsonl`. Include metadata (timestamp, model, branch, first prompt). Create a `internal/session` package.
**Scope**: `new-pkg`, `main` | **Size**: M

### 6.2 Session resume (`/resume`)

**How**: `/resume` command lists sessions from `~/.aql/sessions/` sorted by mtime. Show first prompt + date in a picker. On selection, load the JSONL, reconstruct `[]MessageParam`, and set as agent history.
**Scope**: `tui`, `new-pkg` | **Size**: M

---

## 7. Context Window Management

### 7.1 Auto-compaction

CC automatically compacts when context approaches the limit. AQL only compacts on manual `/compact`.

**How**: In `runner.go`, after each API response, check `tokenCount` against model's context window (e.g. 200k). If above 80% threshold, trigger `CompactHistory()` automatically. Use the existing compact infrastructure.
**Scope**: `agent` | **Size**: S

### 7.2 Precise token counting

AQL estimates tokens from character count. CC tracks exact input/output tokens from API response.

**How**: The Anthropic API returns `usage.input_tokens` and `usage.output_tokens` in the response. Parse these in `runner.go` and send them to the TUI via a new `TokenUsageMsg`. Replace the character-based estimate.
**Scope**: `agent`, `tui` | **Size**: S

---

## 8. Deferred Tool Loading

CC has a `ToolSearch` mechanism — not all tools are loaded into the prompt upfront. The model can request tool schemas on demand.

**How**: Split `ToolDefinitions()` into core (always included: read, edit, write, bash, glob, grep) and deferred (web_fetch, web_search, notebook_edit, etc.). Add a `tool_search` meta-tool that returns schemas for matching tools. When the agent calls a deferred tool, dynamically add it to the tool list for subsequent turns.
**Scope**: `agent` | **Size**: M

---

## 9. MCP (Model Context Protocol)

CC supports connecting to MCP servers that provide additional tools and resources.

**How**: New `internal/mcp` package. Implement the MCP client protocol (JSON-RPC over stdio). At startup, read MCP server configs from `~/.aql/settings.json`. Launch configured servers as child processes. Merge their tool definitions into `ToAPITools()`. Route tool calls to the appropriate MCP server.
**Scope**: `new-pkg`, `agent`, `main` | **Size**: XL

---

## 10. Hook System

CC lets users configure shell commands that run on events (pre-tool-use, post-tool-use, session-start, prompt-submit, stop).

**How**: New `internal/hooks` package. Read hook definitions from `~/.aql/settings.json` under a `hooks` key. Each hook has an event type, a matcher (tool name glob), and a command. In `executeTool()`, run matching pre-hooks before execution and post-hooks after. Hook stdout is parsed as JSON — if it returns `{"decision": "block", "reason": "..."}`, reject the tool call.
**Scope**: `new-pkg`, `agent` | **Size**: L

---

## 11. Sandbox / Command Restrictions

CC sandboxes Bash commands with filesystem and network restrictions.

**How**: Use the existing `bash` tool's `exec.Command`. Before execution, set up filesystem restrictions via environment variables or a wrapper script. Define allowed read/write paths in config. For a simpler v1: add `--read-only` paths and `--deny` paths as config, enforce with path checks before execution (not true sandboxing, but catches common cases).
**Scope**: `agent` | **Size**: L

---

## 12. TUI Features

### 12.1 Image paste / vision support

CC supports pasting images (Ctrl+V) and rendering image tool results.

**How**: Detect image data in clipboard paste events. Base64-encode and send as `image` content block in the API message. The Anthropic SDK already supports `ImageBlockParam`. For display, show `[image: NxN]` placeholder in chat.
**Scope**: `tui`, `agent` | **Size**: M

### 12.2 Diff viewer

CC shows inline diffs when the Edit tool modifies files.

**How**: In the `edit` tool result handler, compute a unified diff between old and new content (use `pmezard/go-difflib`). Render with red/green ANSI coloring in the tool result chat entry. Add a `/diff` command to show the last N file changes.
**Scope**: `tui`, `agent` | **Size**: M

### 12.3 Rewind / undo

CC supports rewinding conversation to a previous checkpoint.

**How**: Store a snapshot of `agent.history` after each user turn (or keep an undo stack of `[]MessageParam` slices). `/rewind` or `Esc Esc` pops the last turn, truncates history, and removes corresponding chat entries from the TUI.
**Scope**: `tui`, `agent` | **Size**: M

### 12.4 Extended thinking toggle

CC supports toggling "extended thinking" mode where the model shows its reasoning.

**How**: Add `Alt+T` keybinding. Toggle a `thinkingEnabled bool` on the model. Pass `thinking` config in `buildMessageParams()` (already partially there for OAuth users). When thinking blocks arrive in the stream, render them in a dimmed/collapsed style.
**Scope**: `tui`, `agent` | **Size**: S

### 12.5 Vim input mode

CC has a `/vim` mode for the input area.

**How**: Bubble Tea has `charmbracelet/bubbles/textinput` which doesn't support vim. Use `charmbracelet/bubbles/textarea` with a vim-mode wrapper or implement basic vi keybindings (i/a/Esc, hjkl, dd, yy, p) as a state machine on the input buffer.
**Scope**: `tui` | **Size**: L

### 12.6 `@` file mentions

CC allows `@filename` in the prompt to auto-include file contents.

**How**: Before sending the prompt, scan for `@<path>` tokens. For each, read the file and prepend its content to the user message as a `<file path="...">` block. Add tab-completion for `@` that globs the working directory.
**Scope**: `tui`, `agent` | **Size**: M

---

## 13. Settings System

CC has `~/.claude/settings.json` for persistent user preferences (permissions, hooks, theme, keybindings).

**How**: New `internal/config` package. Define a `Settings` struct with fields for permission mode, hooks, allowed tools, theme, etc. Load from `~/.aql/settings.json` at startup. Provide a `/config` command to view/edit. Pass settings into agent and TUI initialization.
**Scope**: `new-pkg`, `main`, `tui` | **Size**: M

---

## 14. Skill / Plugin System

CC has a skill/command system where markdown files define prompts that expand on `/skill-name`.

**How**: Scan `~/.aql/plugins/` and `.aql/commands/` for `.md` files with YAML frontmatter. Register each as a slash command. When invoked, read the file, expand template variables, and inject as a user message. Start simple — no agent definitions, no hooks, just command expansion.
**Scope**: `tui`, `new-pkg` | **Size**: L

---

## 15. Scratchpad Directory

CC provides a session-specific temp directory for intermediate files, separate from `/tmp` and the user's project.

**How**: On session start, create `~/.aql/scratchpad/<session-id>/`. Pass the path to the agent as a config field. Mention it in the system prompt. Clean up old scratchpad dirs on startup (older than 7 days).
**Scope**: `main`, `agent` | **Size**: S

---

## 16. Co-authored-by in Commits

CC appends `Co-Authored-By: Claude <model> <noreply@anthropic.com>` to all commits it creates.

**How**: Already in the system prompt instructions for AQL's agent. No code change needed — this is a prompt-level behavior. Verify the agent actually does it by testing a commit flow.
**Scope**: prompt-only | **Size**: S

---

## Priority Order (suggested)

| #   | Feature                                         | Size | Impact                                          |
| --- | ----------------------------------------------- | ---- | ----------------------------------------------- |
| 1   | Git status + env info injection (1.1, 1.2, 1.6) | S    | Agent has crucial context about its environment |
| 2   | Precise token counting (7.2)                    | S    | Accurate cost tracking and compaction triggers  |
| 3   | Dynamic tool descriptions (1.3)                 | S    | System prompt stays in sync with actual tools   |
| 4   | CLAUDE.md hot-reload (1.5)                      | S    | Edit instructions without restarting            |
| 5   | Tool approval prompt (2.1)                      | M    | Safety for write/bash operations                |
| 6   | Diff viewer (12.2)                              | M    | Visibility into what the agent changed          |
| 7   | Task tracking (4)                               | M    | User visibility into multi-step work            |
| 8   | Auto-compaction (7.1)                           | S    | No more manual /compact or context overflow     |
| 9   | Session persistence (6.1, 6.2)                  | M    | Resume interrupted work                         |
| 10  | Rewind (12.3)                                   | M    | Undo bad agent turns                            |
| 11  | Plan mode (5)                                   | M    | Safe exploration before committing to changes   |
| 12  | Sub-agents (3.1) — done                         | L    | Parallel work, research delegation              |
| 13  | `@` file mentions (12.6)                        | M    | Quick file inclusion                            |
| 14  | Memory injection (1.4)                          | M    | Cross-session context                           |
| 15  | Extended thinking toggle (12.4)                 | S    | User control over reasoning visibility          |
| 16  | Settings system (13)                            | M    | Persistent user preferences                     |
| 17  | Hook system (10)                                | L    | Automation and safety guardrails                |
| 18  | Permission modes (2.2)                          | L    | Fine-grained access control                     |
| 19  | Image support (12.1)                            | M    | Vision capabilities                             |
| 20  | MCP support (9)                                 | XL   | Third-party tool ecosystem                      |
| 21  | Sandbox (11)                                    | L    | Command isolation                               |
| 22  | Plugin system (14)                              | L    | Extensibility                                   |
| 23  | Deferred tool loading (8)                       | M    | Smaller default prompt                          |
| 24  | Background agents (3.2)                         | XL   | Non-blocking parallel work                      |
| 25  | Vim mode (12.5)                                 | L    | Power user editing                              |
| 26  | Worktree isolation (3.3)                        | M    | Safe sub-agent file changes                     |
| 27  | Scratchpad directory (15)                       | S    | Clean temp file management                      |
