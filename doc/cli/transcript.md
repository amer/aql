# Transcript View — Spec

## Background

Claude Code's primary conversation display is the **transcript** — a scrolling log that renders user messages, assistant responses, and tool calls with a distinctive visual language: `⏺` filled-circle markers for message blocks, `⎿` connectors for tool results, indented content hierarchy, and collapsible tool groups. AQL currently renders chat entries as flat sequential blocks (user `>` prefix, agent headers, bordered tool boxes) without the visual hierarchy or interactivity that makes Claude Code's transcript scannable and navigable.

This spec covers redesigning AQL's chat rendering into a proper transcript view that matches Claude Code's visual language and adds transcript mode (Ctrl+O) for full-history browsing and search.

## Goals

1. Render conversation with Claude Code's `⏺` / `⎿` visual hierarchy
2. Show tool calls inline with structured input/output display
3. Support collapsing/expanding tool call details
4. Add transcript mode (Ctrl+O) for full-history review and search
5. Group related tool calls (e.g., multiple reads → "Read N files")

## Non-Goals

- Persistent transcript files on disk (defer to session management)
- Subagent/nested transcript display (single-agent only for now)
- Thinking block display (no extended thinking support yet)
- Timestamp markers (add later with `/loop` / cron support)

## Reference: Claude Code Behavior

### Message Rendering

User messages appear as bold text with a `❯` or `>` prompt prefix. Assistant messages use `⏺` filled circles as block markers:

```
⏺ Mouse tracking and native text selection can't coexist in terminals — WithMouseCellMotion
   captures all mouse events including click-drag, which blocks selection. The fix is to drop
   mouse tracking and use keyboard scrolling only.
```

Key elements:

- `⏺` marker in accent color at the start of each assistant block
- Content indented 2 spaces, word-wrapped to terminal width
- Markdown rendered inline (bold, code, lists)

### Tool Call Rendering

Tool calls appear indented under the assistant's message block, with a connector character linking to the result:

```
⏺ Update(cmd/aql/main.go)
  ⎿  Added 3 lines, removed 3 lines
```

Different tool types have different display formats:

**File Read:**

```
⏺ Read(internal/tui/app.go)
  ⎿  845 lines (ctrl+o to expand)
```

**File Edit:**

```
⏺ Update(internal/tui/app.go)
  ⎿  Updated 4 lines
      12  - old line
      13  + new line
```

**Bash:**

```
⏺ Bash(go test ./...)
  ⎿  ok  github.com/amer/aql/internal/tui  0.045s
```

**Search (Grep/Glob):**

```
⏺ Grep(pattern="ToolCall", path="internal/")
  ⎿  Found 12 matches across 4 files
```

### Collapsed Groups

Multiple related tool calls collapse into a summary line:

```
⏺ Read 4 files (ctrl+o to expand)
  ⎿  internal/tui/app.go, internal/tui/styles.go, internal/tui/agent_panel.go, ...
```

While in progress, present tense: "Reading 4 files..."
When complete, past tense: "Read 4 files"

### Transcript Mode (Ctrl+O)

Full-screen mode for reviewing the complete conversation:

- All tool call details expanded
- `/` to search, `n`/`N` to step through matches
- Shows model name per assistant message
- Ctrl+O or Esc to exit back to normal mode

## Design

### Part 1: Transcript Entry Model

Replace the flat `ChatEntry` with a richer transcript-oriented model:

```go
// TranscriptBlock represents a top-level block in the transcript.
// Each block starts with a marker (⏺ for assistant, > for user).
type TranscriptBlock struct {
    Type      BlockType
    AgentName string
    Sections  []BlockSection // ordered content within this block
    Collapsed bool           // whether tool details are collapsed
    Timestamp time.Time
}

type BlockType int

const (
    BlockUser      BlockType = iota // user input
    BlockAssistant                  // assistant response (text + tools)
    BlockSystem                     // system status messages
)

// BlockSection is a piece of content within a block.
type BlockSection struct {
    Type    SectionType
    Text    string     // for text sections
    Tool    *ToolEntry // for tool sections
}

type SectionType int

const (
    SectionText   SectionType = iota // markdown text
    SectionTool                      // tool call + result
    SectionStatus                    // status line (completion time, error)
)

// ToolEntry holds a tool call with its structured result.
type ToolEntry struct {
    Name      string       // tool name: "Read", "Update", "Bash", "Grep", etc.
    Input     string       // display-friendly input summary (file path, command, pattern)
    Result    string       // raw result text
    Summary   string       // one-line summary ("845 lines", "Added 3 lines, removed 3 lines")
    Status    ToolStatus   // Running, Done, Error
    Collapsed bool         // whether this tool's details are hidden
}
```

### Part 2: Transcript Rendering

#### Block-Level Layout

Each block renders with a marker and indented content:

```go
// RenderTranscriptBlock renders a single transcript block.
func RenderTranscriptBlock(block TranscriptBlock, width int, expanded bool) string
```

**User block:**

```
> what files handle tool rendering?
```

**Assistant block:**

```
⏺ The tool rendering is handled by several files in internal/tui/:

  - agent_panel.go — renders tool call blocks with status indicators
  - styles.go — defines visual styles for tool headers and borders

⏺ Read(internal/tui/agent_panel.go)
  ⎿  99 lines

⏺ Read(internal/tui/styles.go)
  ⎿  131 lines
```

**System block (completion, errors):**

```
  ✻ Crunched for 2m15s
```

#### Connector Characters

| Character | Usage                                                      |
| --------- | ---------------------------------------------------------- |
| `⏺`       | Block marker — assistant text blocks and tool call headers |
| `⎿`       | Connector — links tool header to its result                |
| `>`       | User input prefix                                          |
| `✻`       | Completion marker                                          |

#### Tool Header Formatting

Tool headers follow the pattern `Name(input_summary)`:

| Tool           | Header Format         | Example                         |
| -------------- | --------------------- | ------------------------------- |
| read_file      | `Read(path)`          | `Read(internal/tui/app.go)`     |
| write_file     | `Write(path)`         | `Write(internal/tui/diff.go)`   |
| edit (update)  | `Update(path)`        | `Update(internal/tui/app.go)`   |
| bash           | `Bash(command)`       | `Bash(go test ./...)`           |
| grep           | `Grep(pattern, path)` | `Grep("ToolCall", internal/)`   |
| list_directory | `List(path)`          | `List(internal/tui/)`           |
| web_fetch      | `Fetch(url)`          | `Fetch(https://example.com)`    |
| web_search     | `Search(query)`       | `Search("bubbletea scrolling")` |

Long inputs are truncated to fit within terminal width with `...`.

#### Tool Result Summary

Each tool type produces a one-line summary for the collapsed view:

| Tool           | Summary Format                   |
| -------------- | -------------------------------- |
| read_file      | `N lines`                        |
| write_file     | `Wrote N lines`                  |
| edit           | `Added N lines, removed M lines` |
| bash           | First line of output (truncated) |
| grep           | `Found N matches across M files` |
| list_directory | `N items`                        |
| web_fetch      | `Fetched N bytes`                |
| web_search     | `N results`                      |

#### Collapsed vs Expanded

**Collapsed** (default for completed tools):

```
⏺ Read(internal/tui/app.go)
  ⎿  845 lines (ctrl+o to expand)
```

**Expanded** (in transcript mode or during streaming):

```
⏺ Read(internal/tui/app.go)
  ⎿  1  package tui
     2
     3  import (
     4      "fmt"
     ...
     845 }
```

Expansion hint `(ctrl+o to expand)` shown in dim style only when collapsed.

### Part 3: Tool Group Collapsing

Consecutive tool calls of the same type collapse into a group:

```go
// ToolGroup represents consecutive tool calls that can be collapsed together.
type ToolGroup struct {
    ToolName string       // e.g., "Read", "Grep"
    Entries  []ToolEntry
    Active   bool         // still accumulating entries
}
```

#### Grouping Rules

- Only consecutive same-type tools group (text between breaks the group)
- Groups of 1 tool are not collapsed (show normally)
- Groups of 2+ tools collapse into a summary line

#### Group Rendering

**While active (streaming):**

```
⏺ Reading 3 files...
  ⎿  internal/tui/app.go, internal/tui/styles.go, internal/tui/agent_panel.go
```

**When complete:**

```
⏺ Read 4 files
  ⎿  internal/tui/app.go, internal/tui/styles.go, internal/tui/agent_panel.go, +1 more
```

File list truncated to fit terminal width, with `+N more` suffix.

### Part 4: Transcript Mode (Ctrl+O)

A full-screen overlay for reviewing the complete conversation.

#### State

```go
type TranscriptMode struct {
    Active       bool
    ScrollOffset int
    SearchQuery  string
    SearchActive bool          // currently typing search
    Matches      []SearchMatch // positions of search hits
    MatchIndex   int           // current match (for n/N navigation)
}

type SearchMatch struct {
    BlockIndex   int
    SectionIndex int
    ByteOffset   int
}
```

#### Key Bindings

| Key               | Action                              |
| ----------------- | ----------------------------------- |
| Ctrl+O            | Toggle transcript mode on/off       |
| Esc               | Exit transcript mode                |
| /                 | Start search                        |
| n                 | Next search match                   |
| N (shift+n)       | Previous search match               |
| Enter (in search) | Confirm search, jump to first match |
| Esc (in search)   | Cancel search                       |
| Up/Down           | Scroll line by line                 |
| PgUp/PgDown       | Scroll by half page                 |
| Home              | Jump to top                         |
| End               | Jump to bottom                      |
| Ctrl+U            | Half page up                        |
| Ctrl+D            | Half page down                      |

#### Rendering Differences from Normal Mode

- All tool calls expanded (full output visible)
- No prompt area at bottom (read-only mode)
- Header shows "Transcript (Ctrl+O to exit)" instead of normal header
- Search matches highlighted with inverse video or accent background
- Status bar shows match count: "Match 3/12" or "No matches"
- Model name displayed next to each assistant block marker

#### Search

Search is case-insensitive substring match across all rendered text content. Matches highlight within the rendered transcript. The view auto-scrolls to center the current match.

### Part 5: Migration from ChatEntry

The existing `ChatEntry` model is replaced by `TranscriptBlock`. Migration path:

1. `EntryUserInput` → `BlockUser` with single `SectionText`
2. `EntryAgentText` → `BlockAssistant` with `SectionText` (accumulates streaming deltas into current block)
3. `EntryAgentTool` → `SectionTool` appended to current `BlockAssistant`
4. `EntryAgentStatus` → `SectionStatus` appended to current block, or `BlockSystem` for standalone status

Key change: instead of appending a new entry for every event, streaming deltas and tool calls accumulate into the current assistant block. A new block starts only when the speaker changes (user → assistant, assistant → user).

#### Streaming Behavior

During streaming, the current assistant block grows:

1. `AgentStreamStartMsg` → create new `BlockAssistant`
2. `AgentStreamDeltaMsg` → append to current block's last `SectionText`
3. `AgentToolCallMsg` → append `SectionTool` to current block
4. `AgentStreamDoneMsg` → append `SectionStatus` with completion time

Tool calls that arrive mid-stream create a new section within the same block, so text and tools interleave naturally.

## Implementation Plan

### Phase 1: TranscriptBlock data model

**Test first:**

```go
func TestTranscriptBlock_UserBlock(t *testing.T)
func TestTranscriptBlock_AssistantWithText(t *testing.T)
func TestTranscriptBlock_AssistantWithTools(t *testing.T)
func TestTranscriptBlock_AppendDelta(t *testing.T)
func TestTranscriptBlock_AppendTool(t *testing.T)
```

**Files:** `internal/tui/transcript.go`, `internal/tui/transcript_test.go`

### Phase 2: Transcript rendering functions

**Test first:**

```go
func TestRenderUserBlock(t *testing.T)
func TestRenderAssistantBlock_TextOnly(t *testing.T)
func TestRenderAssistantBlock_WithTools(t *testing.T)
func TestRenderToolEntry_Collapsed(t *testing.T)
func TestRenderToolEntry_Expanded(t *testing.T)
func TestRenderToolEntry_Running(t *testing.T)
func TestRenderToolEntry_Error(t *testing.T)
func TestRenderSystemBlock(t *testing.T)
func TestRenderConnector(t *testing.T)
```

**Files:** `internal/tui/transcript.go`, `internal/tui/styles.go`

### Phase 3: Tool header formatting and summaries

**Test first:**

```go
func TestToolHeader_ReadFile(t *testing.T)
func TestToolHeader_Bash(t *testing.T)
func TestToolHeader_Grep(t *testing.T)
func TestToolHeader_LongInputTruncation(t *testing.T)
func TestToolSummary_ReadFile(t *testing.T)
func TestToolSummary_Edit(t *testing.T)
func TestToolSummary_Bash(t *testing.T)
func TestToolSummary_Grep(t *testing.T)
```

**Files:** `internal/tui/transcript.go`, `internal/tui/transcript_test.go`

### Phase 4: Tool group collapsing

**Test first:**

```go
func TestToolGroup_ConsecutiveReads(t *testing.T)
func TestToolGroup_SingleToolNoCollapse(t *testing.T)
func TestToolGroup_MixedToolsNoCollapse(t *testing.T)
func TestToolGroup_ActiveVsComplete(t *testing.T)
func TestToolGroup_RenderCollapsed(t *testing.T)
func TestToolGroup_FileListTruncation(t *testing.T)
```

**Files:** `internal/tui/transcript.go`, `internal/tui/transcript_test.go`

### Phase 5: Migrate Model from ChatEntry to TranscriptBlock

**Test first:**

```go
func TestModel_StreamDeltaAccumulatesInBlock(t *testing.T)
func TestModel_ToolCallAppendsToCurrentBlock(t *testing.T)
func TestModel_UserInputStartsNewBlock(t *testing.T)
func TestModel_StreamDoneAddsStatus(t *testing.T)
func TestModel_ViewRendersTranscript(t *testing.T)
```

**Files:** `internal/tui/app.go`, `internal/tui/app_test.go`

### Phase 6: Transcript mode (Ctrl+O)

**Test first:**

```go
func TestTranscriptMode_ToggleOnOff(t *testing.T)
func TestTranscriptMode_ExpandsAllTools(t *testing.T)
func TestTranscriptMode_Scrolling(t *testing.T)
func TestTranscriptMode_EscExits(t *testing.T)
func TestTranscriptMode_BlocksInput(t *testing.T)
```

**Files:** `internal/tui/app.go`, `internal/tui/app_test.go`

### Phase 7: Transcript search

**Test first:**

```go
func TestTranscriptSearch_FindsMatches(t *testing.T)
func TestTranscriptSearch_CaseInsensitive(t *testing.T)
func TestTranscriptSearch_NextPrevious(t *testing.T)
func TestTranscriptSearch_NoMatches(t *testing.T)
func TestTranscriptSearch_HighlightsMatch(t *testing.T)
func TestTranscriptSearch_ScrollsToMatch(t *testing.T)
```

**Files:** `internal/tui/transcript.go`, `internal/tui/transcript_test.go`

## Styles

Add to `styles.go`:

```go
// Transcript markers
TranscriptMarkerStyle  // ⏺ — accent color, bold
TranscriptConnectorStyle // ⎿ — dim color
TranscriptIndentStyle  // 2-space indent for block content

// Tool display
ToolNameStyle      // tool name in header — accent color, bold
ToolInputStyle     // input params — text color
ToolSummaryStyle   // one-line result summary — muted, italic
ToolHintStyle      // "(ctrl+o to expand)" — dim

// Search
SearchHighlightStyle // inverse video or accent background for matches
SearchBarStyle       // search input at bottom of transcript mode

// Transcript mode header
TranscriptModeHeader // "Transcript" label — accent, bold
```

## Files to Change

| File                               | Changes                                                                                                                          |
| ---------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `internal/tui/transcript.go`       | New — TranscriptBlock, BlockSection, ToolEntry, ToolGroup types, rendering functions, search                                     |
| `internal/tui/transcript_test.go`  | New — all transcript rendering and logic tests                                                                                   |
| `internal/tui/styles.go`           | Add transcript marker, connector, tool display, and search styles                                                                |
| `internal/tui/app.go`              | Replace `chat []ChatEntry` with `blocks []TranscriptBlock`, add TranscriptMode state, handle Ctrl+O, update all message handlers |
| `internal/tui/app_test.go`         | Update tests for new transcript model                                                                                            |
| `internal/tui/agent_panel.go`      | Deprecate or remove — functionality moves to transcript.go                                                                       |
| `internal/tui/commands.go`         | No changes needed (Ctrl+O is a keybinding, not a slash command)                                                                  |
| `internal/tui/integration_test.go` | Update for transcript block model                                                                                                |

## Open Questions

1. **Collapse threshold?** Claude Code collapses groups of 2+ same-type tools. Should we match that or require 3+? **Recommendation:** 3+ to avoid over-collapsing — pairs of reads are common and useful to see individually.

2. **Tool result content in normal mode?** Should completed tool results show the first few lines by default, or only the summary? **Recommendation:** summary-only with `(ctrl+o to expand)` hint — keeps the transcript scannable. Bash tool is the exception: show first 5 lines since command output is often the answer itself.

3. **Block boundaries during streaming?** If the assistant sends text, then a tool call, then more text, that's 3 sections in 1 block. But what if there's a long pause between? **Recommendation:** keep it as one block — the block boundary is the speaker change, not timing.

4. **Search scope in transcript mode?** Search all text including tool results, or only assistant text? **Recommendation:** search everything — tool output often contains what you're looking for.

5. **Backward compatibility?** The `ChatEntry` type is used in tests and the public `Chat()` accessor. **Recommendation:** keep `Chat()` working by converting blocks back to entries, or provide a new `Blocks()` accessor and deprecate `Chat()`. Prefer the latter — cleaner break.
