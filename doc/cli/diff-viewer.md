# Diff Viewer — Spec

## Background

Claude Code displays file changes inline with color-coded diffs (green additions, red deletions) and provides an interactive `/diff` command to browse all changes across turns. AQL currently renders tool calls as simple bordered blocks with status icons. This spec covers both inline diff rendering in tool results and the interactive `/diff` dialog.

## Goals

1. Render file edit tool results with color-coded inline diffs
2. Implement `/diff` slash command with interactive file browser
3. Track per-turn file changes for turn-by-turn diff navigation
4. Support both git diff and per-turn diff views

## Non-Goals

- Side-by-side diff view (terminal width makes this impractical — defer to IDE integration)
- Syntax highlighting within diffs (glamour handles markdown; raw diff lines are enough)
- Image diffing

## Reference: Claude Code Behavior

### Inline Tool Result Diffs

When Claude Code edits a file, the tool result shows:

```
⏺ Update(internal/tui/app.go)
  ⎿  Updated 4 lines
      12  - old line
      13  + new line
      14  + another new line
```

Key elements:

- `⏺` filled circle header with tool action + file path
- `⎿` connector character linking header to content
- Line numbers in dim/muted color
- `-` lines in red (deletions)
- `+` lines in green (additions)
- Context lines (unchanged) in muted color
- Line count summary ("Updated 4 lines", "Added 12 lines", "Removed 3 lines")

### /diff Dialog

- Left/right arrows switch between: **git diff** (all uncommitted changes) and **per-turn diffs** (changes from each Claude response)
- Up/down arrows navigate between modified files
- Enter to view file details
- Escape to dismiss
- Shows file paths with change indicators (+/- line counts)

## Design

### Part 1: Enriched ToolCall Model

Extend `ToolCall` to carry structured diff data when the tool is a file edit:

```go
// DiffLine represents a single line in a unified diff.
type DiffLine struct {
    Type    DiffLineType
    LineNo  int    // line number in the new file (0 if deletion-only)
    Content string
}

type DiffLineType int

const (
    DiffContext  DiffLineType = iota // unchanged line
    DiffAdd                          // added line
    DiffDelete                       // removed line
)

// FileDiff holds the parsed diff for a single file edit.
type FileDiff struct {
    Path      string
    Action    string     // "Update", "Write", "Delete"
    Lines     []DiffLine
    AddCount  int
    DelCount  int
}

// ToolCall represents a tool invocation to display.
type ToolCall struct {
    Name    string
    Content string
    Status  ToolStatus
    Diff    *FileDiff  // non-nil for file edit tools
}
```

### Part 2: Inline Diff Rendering

Replace the plain `RenderToolBlock` output for file edits with a Claude Code-style diff display.

#### Rendering Format

```
  ╭──
  │ ✓ Update(internal/tui/app.go)
  │ ⎿  Updated 4 lines
  │     12    context line
  │     13  - old line
  │     14  + new line
  │     15  + another new line
  │     16    context line
  ╰──
```

#### Styles

Add to `styles.go`:

```go
DiffAddStyle    = lipgloss.NewStyle().Foreground(successColor) // green
DiffDeleteStyle = lipgloss.NewStyle().Foreground(errorColor)   // red
DiffContextStyle = lipgloss.NewStyle().Foreground(dimColor)    // muted
DiffLineNoStyle = lipgloss.NewStyle().Foreground(mutedColor)   // line numbers
DiffHeaderStyle = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
DiffSummaryStyle = lipgloss.NewStyle().Foreground(mutedColor).Italic(true)
```

#### Rendering Function

```go
// RenderFileDiff renders a file diff in Claude Code style.
func RenderFileDiff(diff FileDiff, status ToolStatus) string
```

Logic:

1. Render header: status icon + action + file path
2. Render summary: "Updated N lines" / "Added N lines" / "Removed N lines"
3. Render diff lines with line numbers, +/- prefixes, and color coding
4. Wrap in tool block border

#### Truncation

- Show at most 20 diff lines by default
- If truncated, show `... +N more lines` in dim style
- Full content available via `/diff` dialog

### Part 3: Turn Tracking

Track which files changed in each turn for per-turn diff navigation.

```go
// TurnDiff records file changes from a single assistant turn.
type TurnDiff struct {
    TurnIndex int        // index in chat entries
    Files     []FileDiff // files changed in this turn
}
```

The Model gains:

```go
type Model struct {
    // ... existing fields ...
    turnDiffs []TurnDiff // accumulated per-turn diffs
}
```

On each `AgentToolCallMsg` with a non-nil `Diff`, append to the current turn's diff list. A new turn starts when a user submits input.

### Part 4: /diff Dialog

Interactive dialog opened by the `/diff` slash command.

#### State

```go
type DiffDialogState struct {
    Visible   bool
    Mode      DiffMode       // DiffModeGit or DiffModeTurn
    FileIdx   int            // selected file index
    Files     []FileDiff     // files in current view
    TurnIdx   int            // selected turn (turn mode only)
}

type DiffMode int

const (
    DiffModeGit  DiffMode = iota // git diff of all uncommitted changes
    DiffModeTurn                  // per-turn diffs from chat history
)
```

#### Key Bindings (when dialog is visible)

| Key        | Action                                |
| ---------- | ------------------------------------- |
| Left/Right | Switch between git diff and turn mode |
| Up/Down    | Navigate file list                    |
| Enter      | View selected file's full diff        |
| Escape     | Close dialog                          |

#### Git Diff Source

Execute `git diff` via the existing `BashFunc` mechanism and parse the unified diff output into `[]FileDiff`.

#### Rendering

```
╭─ Diff Viewer ─────────────────────────────────────────────╮
│ [Git Diff]  Per-Turn                                      │
│                                                           │
│ > internal/tui/app.go          +15  -3                    │
│   internal/tui/styles.go       +6   -0                    │
│   internal/tui/agent_panel.go  +42  -8                    │
│                                                           │
│ ← → switch mode  ↑ ↓ navigate  Enter view  Esc close     │
╰───────────────────────────────────────────────────────────╯
```

- Active mode tab is highlighted (bold + accent color)
- Selected file has `>` prefix in brand color
- +/- counts in green/red respectively
- Key hints at bottom in dim style

#### Detail View (after Enter)

Shows the full diff for the selected file using `RenderFileDiff` without truncation, scrollable with the existing scroll mechanism.

### Part 5: Diff Parsing

Parse unified diff format (from `git diff` output) into `[]FileDiff`:

```go
// ParseUnifiedDiff parses git unified diff output into structured FileDiffs.
func ParseUnifiedDiff(raw string) []FileDiff
```

Handles:

- `diff --git a/path b/path` — file boundary
- `@@ -start,count +start,count @@` — hunk headers
- `-` lines — deletions
- `+` lines — additions
- ` ` lines — context
- Binary file detection (skip)

## Implementation Plan

### Phase 1: DiffLine and FileDiff types + parsing

**Test first:**

```go
func TestParseDiffLine_Add(t *testing.T)
func TestParseDiffLine_Delete(t *testing.T)
func TestParseDiffLine_Context(t *testing.T)
func TestParseUnifiedDiff_SingleFile(t *testing.T)
func TestParseUnifiedDiff_MultipleFiles(t *testing.T)
func TestParseUnifiedDiff_Empty(t *testing.T)
```

**Files:** `internal/tui/diff.go`, `internal/tui/diff_test.go`

### Phase 2: Inline diff rendering

**Test first:**

```go
func TestRenderFileDiff_AdditionsOnly(t *testing.T)
func TestRenderFileDiff_DeletionsOnly(t *testing.T)
func TestRenderFileDiff_Mixed(t *testing.T)
func TestRenderFileDiff_Truncated(t *testing.T)
func TestRenderFileDiff_EmptyDiff(t *testing.T)
```

**Files:** `internal/tui/diff.go`, `internal/tui/styles.go`

### Phase 3: ToolCall with Diff field

**Test first:**

```go
func TestRenderToolBlock_WithDiff(t *testing.T)
func TestRenderToolBlock_WithoutDiff(t *testing.T) // existing behavior preserved
```

**Files:** `internal/tui/agent_panel.go`, `internal/tui/app.go`

### Phase 4: Turn tracking

**Test first:**

```go
func TestTurnDiff_AccumulatesAcrossTurns(t *testing.T)
func TestTurnDiff_ResetOnClear(t *testing.T)
func TestTurnDiff_NewTurnOnUserSubmit(t *testing.T)
```

**Files:** `internal/tui/app.go`

### Phase 5: /diff dialog — git diff mode

**Test first:**

```go
func TestDiffDialog_OpensOnSlashDiff(t *testing.T)
func TestDiffDialog_EscCloses(t *testing.T)
func TestDiffDialog_UpDownNavigatesFiles(t *testing.T)
func TestDiffDialog_LeftRightSwitchesMode(t *testing.T)
func TestDiffDialog_RendersFileList(t *testing.T)
```

**Files:** `internal/tui/diff.go`, `internal/tui/app.go`, `internal/tui/commands.go`

### Phase 6: /diff dialog — per-turn mode

**Test first:**

```go
func TestDiffDialog_TurnModeShowsTurnDiffs(t *testing.T)
func TestDiffDialog_TurnNavigation(t *testing.T)
```

**Files:** `internal/tui/diff.go`, `internal/tui/app.go`

## Files to Change

| File                               | Changes                                                                                                 |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------- |
| `internal/tui/diff.go`             | New — DiffLine, FileDiff, TurnDiff types, ParseUnifiedDiff, RenderFileDiff                              |
| `internal/tui/diff_test.go`        | New — parsing and rendering tests                                                                       |
| `internal/tui/styles.go`           | Add DiffAddStyle, DiffDeleteStyle, DiffContextStyle, DiffLineNoStyle, DiffHeaderStyle, DiffSummaryStyle |
| `internal/tui/agent_panel.go`      | Extend ToolCall with Diff field, update RenderToolBlock to use RenderFileDiff                           |
| `internal/tui/app.go`              | Add turnDiffs, DiffDialogState, handle /diff dialog keys, track turns                                   |
| `internal/tui/commands.go`         | Add /diff to slash commands                                                                             |
| `internal/tui/app_test.go`         | Dialog interaction tests                                                                                |
| `internal/tui/integration_test.go` | End-to-end diff flow tests                                                                              |

## Open Questions

1. **Context lines around changes?** Git diff shows 3 context lines by default. We could show 1-2 to save space. **Recommendation:** show 2 context lines.

2. **Diff for Write (new file) vs Edit (patch)?** Write creates the whole file — showing every line as `+` is noisy. **Recommendation:** for new files, show first 10 lines with `... +N more lines`; for edits, show the full diff.

3. **Git diff execution timing?** Running `git diff` on `/diff` open is simplest but could be slow on large repos. **Recommendation:** run on demand (when dialog opens), show a spinner while loading.

4. **File path display?** Full paths can be long. **Recommendation:** show paths relative to project root, truncate from the left if wider than available space.
