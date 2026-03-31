# Sessions Spec

Reference: Claude Code session management (v2.1.87).

## Overview

A **session** is a conversation between user and agent, persisted as a JSONL transcript on disk. Sessions can be created, resumed, forked, named, compacted, and auto-cleaned.

## Session ID

- UUID v4 generated via `crypto.randomUUID()` (Go equivalent: `uuid.New()`)
- Created at startup or when explicitly forking/creating a new session
- Immutable for the lifetime of a session (resume reuses the same ID)

## Disk Layout

All session data lives under `~/.claude/` (or equivalent config dir).

```
~/.claude/
  projects/<path-slug>/
    <session-id>.jsonl          # full conversation transcript
    <session-id>/
      subagents/                # sub-agent transcripts
        <agent-id>.jsonl
        <agent-id>.meta.json
      tool-results/             # cached tool output
        <short-id>.txt
    memory/                     # cross-session project memory
      MEMORY.md
      *.md
  sessions/<pid>.json           # PID-to-session registry (active processes)
  session-env/<session-id>/     # per-session env var overrides
  file-history/<session-id>/    # file backups for undo/rewind
  tasks/<session-id>/           # background task data
  history.jsonl                 # global input history
  shell-snapshots/              # shell environment captures
```

**Path slug**: working directory with `/` replaced by `-`, e.g. `-Users-Amer-Code-github-com-amer-aql`.

## Transcript Format (JSONL)

Each line is a JSON object. Key fields shared across types:

| Field        | Description                              |
| ------------ | ---------------------------------------- |
| `type`       | Entry type (see below)                   |
| `uuid`       | Unique ID for this entry                 |
| `parentUuid` | Links assistant replies to user messages |
| `sessionId`  | Session this entry belongs to            |
| `timestamp`  | ISO 8601 timestamp                       |
| `version`    | CLI version that wrote this entry        |
| `gitBranch`  | Active git branch at time of entry       |

### Entry Types

| Type                      | Purpose                                       |
| ------------------------- | --------------------------------------------- |
| `user`                    | User message                                  |
| `assistant`               | Model response (includes `model`, `id`)       |
| `file-history-snapshot`   | File state for undo/rewind                    |
| `summary`                 | Session summary (for picker UI)               |
| `custom-title`            | User-set session title                        |
| `compaction`              | Context compaction metadata                   |
| `content-replacement`     | Replaced content (e.g. truncated tool output) |
| `context-collapse-commit` | Context window collapse point                 |
| `tag`                     | User-applied tag                              |
| `pr-link`                 | Associated pull request                       |
| `attribution-snapshot`    | Attribution data                              |
| `agent-setting`           | Agent configuration change                    |
| `worktree-state`          | Git worktree state                            |
| `speculation-accept`      | Accepted speculative content                  |

### User Message Example

```json
{
  "type": "user",
  "uuid": "8703054d-...",
  "parentUuid": null,
  "isSidechain": false,
  "promptId": "0080095c-...",
  "message": { "role": "user", "content": "..." },
  "timestamp": "2026-03-30T17:08:48.774Z",
  "permissionMode": "default",
  "userType": "external",
  "entrypoint": "cli",
  "cwd": "/Users/Amer/Code/github.com/amer/aql",
  "sessionId": "08652836-...",
  "version": "2.1.87",
  "gitBranch": "main"
}
```

### Assistant Message Example

```json
{
  "type": "assistant",
  "uuid": "f3ef28ba-...",
  "parentUuid": "8703054d-...",
  "isSidechain": false,
  "message": {"model": "claude-opus-4-6", "id": "msg_...", "role": "assistant", "content": [...]},
  "requestId": "req_...",
  "timestamp": "2026-03-30T17:08:53.201Z",
  "sessionId": "08652836-...",
  "version": "2.1.87",
  "gitBranch": "main"
}
```

## PID Registry

`~/.claude/sessions/<pid>.json` maps a running process to its session:

```json
{
  "pid": 65274,
  "sessionId": "08652836-2f33-4c0f-b40b-5fe4dfdf5b81",
  "cwd": "/Users/Amer/Code/github.com/amer/aql",
  "startedAt": 1774890317846,
  "kind": "interactive",
  "entrypoint": "cli",
  "name": "optional-human-readable-name"
}
```

- `name` is optional, set via `--name` flag or `/rename` command
- Stale entries (PID no longer running) are cleaned up on startup

## Session Lifecycle

### Create

1. Generate UUID
2. Initialize session state (cwd, project dir, permissions, etc.)
3. Write PID registry entry
4. Capture shell environment snapshot
5. Begin appending to `<session-id>.jsonl`

### Resume (`--continue` / `--resume`)

**`--continue` (`-c`)**:

- Continues the most recent session **in the current directory**
- Lists all transcript files for the project, sorted by mtime
- Picks the most recent non-sidechain session
- Reuses the original session ID (appends to same `.jsonl`)

**`--resume` (`-r`)**:

- Without argument: opens interactive fuzzy-search session picker
- With session ID: resumes that specific session (UUID or prefix match)
- With search term: searches session titles/summaries
- Also reuses the original session ID

**`--resume-session-at <message-id>`** (hidden):

- Loads session but truncates to messages up to the specified assistant message UUID
- Used for "rewinding" a conversation

**`--rewind-files <user-message-id>`** (hidden):

- Restores files to state at given user message, then exits
- Requires `--resume`

### Fork (`--fork-session`)

- Used with `--resume` or `--continue`
- Creates a **new session ID** instead of reusing the original
- Old conversation history is loaded but a fresh `.jsonl` is started
- `--session-id` can only combine with `--continue`/`--resume` if `--fork-session` is also set

### Name (`--name` / `/rename`)

- Sets human-readable display name
- Stored in PID registry JSON
- Visible in resume picker and terminal title

### No Persistence (`--no-session-persistence`)

- Only works with `--print` (headless/pipe mode)
- No transcript written to disk; session cannot be resumed

## Context Compaction

Sessions are not limited by a fixed context window. Claude Code manages context through auto-compaction:

1. **Micro-compaction**: lightweight pre-pass, runs first
2. **Auto-compaction**: when token count approaches model limit, summarizes older messages into a compact summary
3. Compacted summary replaces older messages in API requests
4. Full transcript on disk always contains ALL messages (compaction only affects what's sent to API)
5. Compaction metadata stored as `type: "compaction"` entries in transcript

## Session Cleanup

- Controlled by `cleanupPeriodDays` setting (default: **30 days**)
- At startup, transcripts older than cutoff are deleted
- `cleanupPeriodDays: 0` disables persistence entirely (no transcripts written, existing ones deleted)
- Stale PID registry files (process no longer running) are cleaned up

## Sub-agents

- Parent session tracks `parentSessionId` in addition to `sessionId`
- Sub-agent transcripts stored in `<session-id>/subagents/`
- Each sub-agent has `.jsonl` (transcript) and `.meta.json` (metadata)
- Meta contains: `{"agentType": "general-purpose", "description": "..."}`

## Session State

Runtime state tracked per session:

| Field                          | Purpose                            |
| ------------------------------ | ---------------------------------- |
| `sessionId`                    | Current session UUID               |
| `parentSessionId`              | Parent session (if sub-agent)      |
| `sessionProjectDir`            | Project directory for this session |
| `originalCwd`                  | Working directory at session start |
| `projectRoot`                  | Detected project root              |
| `sessionBypassPermissionsMode` | Permission overrides               |
| `sessionPersistenceDisabled`   | Whether transcript writing is off  |
| `sessionCronTasks`             | Recurring tasks for this session   |
| `promptId`                     | Current prompt turn ID             |
| `lastAPIRequest`               | Last API request for debugging     |
| `pendingPostCompaction`        | Pending compaction work            |

## Cross-Project / Teleport

- **Teleport** (`--teleport`): resumes a remote session from claude.ai web into CLI
- **Cross-project resume**: when resuming from a different project dir, outputs `cd <dir> && claude --resume <id>` instead of resuming directly
- **`--from-pr`**: links/resumes sessions associated with a specific pull request

## CLI Flags Summary

| Flag                       | Description                                     |
| -------------------------- | ----------------------------------------------- |
| `--continue`, `-c`         | Resume most recent session in current dir       |
| `--resume`, `-r`           | Resume by ID, prefix, search, or interactive    |
| `--session-id`             | Specify custom UUID (requires `--fork-session`) |
| `--fork-session`           | Create new ID when resuming                     |
| `--name`, `-n`             | Set human-readable session name                 |
| `--no-session-persistence` | Don't write transcript (print mode only)        |
| `--resume-session-at`      | Rewind to specific message (hidden)             |
| `--rewind-files`           | Restore files to message state (hidden)         |
| `--teleport`               | Resume remote session from web                  |
| `--from-pr`                | Resume session linked to a PR                   |
