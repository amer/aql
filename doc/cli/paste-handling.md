# Paste Handling — Spec

## Background

Claude Code collapses pasted text into `[Pasted text #N +M lines]` placeholder chips. This keeps the input area clean when pasting large code blocks or logs, while preserving the full content for submission. AQL should implement similar paste handling using Bubble Tea's built-in bracketed paste support.

## Goals

1. Detect pasted text vs typed input
2. Collapse multi-line pastes into numbered placeholder chips
3. Allow users to review paste content before submission
4. Handle edge cases (empty paste, single-line, rapid sequential pastes)
5. Keep the InputBuffer testable as a pure data structure

## Non-Goals

- Image paste (Ctrl+V clipboard image) — separate feature
- Paste preprocessing (auto-trimming, reformatting) — keep raw content
- Configurable collapse threshold — hardcode a sensible default, revisit later

## Design

### How Bubble Tea Delivers Pastes

Bubble Tea detects bracketed paste sequences (`\e[200~` ... `\e[201~`) automatically. When a paste arrives, the `tea.KeyMsg` has:

```go
tea.KeyMsg{
    Type:  tea.KeyRunes,
    Runes: []rune("the pasted text including\nnewlines"),
    Paste: true,  // <-- distinguishes paste from typed input
}
```

Single-character pastes (`Paste: true` but `len(Runes) == 1`) should be treated as normal typed input — no collapse needed.

### Collapse Threshold

Collapse when **both** conditions are met:

- `Paste: true` on the KeyMsg
- Content contains at least 2 newlines OR exceeds 200 characters

Single-line short pastes insert inline as regular text.

### Data Model

Add a paste store to the Model, parallel to the chat log:

```go
// PastedText holds the full content of a collapsed paste.
type PastedText struct {
    ID      int    // sequential, 1-based
    Content string // raw pasted text
    Lines   int    // line count for display
}
```

The Model gains:

```go
type Model struct {
    // ... existing fields ...
    pastes     []PastedText // stored pastes for current input
    pasteCount int          // counter for paste IDs (reset on submit/clear)
}
```

### Input Flow

```
User pastes multi-line text
         │
         ▼
  tea.KeyMsg{Paste: true}
         │
         ▼
  Exceeds collapse threshold?
    │              │
   No             Yes
    │              │
    ▼              ▼
 Insert as      Store in pastes[]
 plain text     Insert chip marker
                "[Pasted text #N +M lines]"
                into InputBuffer
```

### Chip Markers in InputBuffer

When a paste is collapsed, insert a placeholder string into the InputBuffer:

```
[Pasted text #1 +14 lines]
```

The marker is a literal string in the buffer. The user can see it, cursor over it, and delete it (which also removes the stored paste).

On submit, expand all markers back to their full content before sending to `onSubmit`. The expansion happens in the submit handler, not in the InputBuffer itself.

### Rendering

The chip marker renders with a distinct style (e.g., dimmed or inverse) in the prompt area. Detection is by regex pattern matching during `RenderPromptArea`:

```
\[Pasted text #\d+ \+\d+ lines\]
```

### Submit Expansion

When the user presses Enter:

1. Get the raw input string from InputBuffer
2. For each `[Pasted text #N +M lines]` marker, replace with the stored `pastes[N-1].Content`
3. Pass the expanded string to `onSubmit`
4. Clear `pastes` and reset `pasteCount`

### Deletion Behavior

- If the user backspaces through a chip marker character-by-character, the marker text shrinks like normal text. Once fully deleted, the stored paste is orphaned (harmless — cleared on submit).
- If the user does Ctrl+U (kill to start) or Ctrl+K (kill to end) and it removes a marker, same behavior — orphaned paste is fine.
- `/clear` resets pastes along with everything else.

### Edge Cases

| Case                          | Behavior                                           |
| ----------------------------- | -------------------------------------------------- |
| Empty paste (`""`)            | Ignore, no-op                                      |
| Single character paste        | Insert as normal typed input                       |
| Single line, < 200 chars      | Insert as normal typed input                       |
| Multi-line, 2+ newlines       | Collapse into chip                                 |
| Single line, >= 200 chars     | Collapse into chip                                 |
| Rapid sequential pastes       | Each gets its own numbered chip                    |
| Paste while streaming         | Buffer accepts paste, blocked from submit as usual |
| Paste in middle of typed text | Chip inserted at cursor position                   |

### Keyboard Shortcuts

No new shortcuts needed initially. The paste is received via the terminal's native Ctrl+V/Cmd+V and detected through Bubble Tea's bracketed paste.

Future: consider a shortcut to expand/preview a chip inline (Claude Code users frequently request this — issues #11033, #23134, #35581).

## Implementation Plan

### Phase 1: InputBuffer — InsertString method

**Test first:**

```go
func TestInputBuffer_InsertString(t *testing.T) {
    buf := tui.NewInputBuffer()
    buf.InsertString("hello world")
    assert.Equal(t, "hello world", buf.String())
    assert.Equal(t, 11, buf.Cursor())
}

func TestInputBuffer_InsertStringAtMiddle(t *testing.T) {
    buf := tui.NewInputBuffer()
    buf.InsertString("hd")
    buf.MoveLeft()
    buf.InsertString("ello worl")
    assert.Equal(t, "hello world", buf.String())
}
```

**Implement:** Add `InsertString(s string)` to InputBuffer — insert each rune at cursor.

### Phase 2: Paste detection and storage

**Test first:**

```go
func TestPaste_MultiLineCollapsed(t *testing.T) {
    m := testModel(nil)
    m = applyPaste(m, "line1\nline2\nline3")
    assert.Contains(t, m.Input(), "[Pasted text #1 +3 lines]")
    assert.Equal(t, 1, m.PasteCount())
}

func TestPaste_ShortInlineInsert(t *testing.T) {
    m := testModel(nil)
    m = applyPaste(m, "short text")
    assert.Equal(t, "short text", m.Input())
    assert.Equal(t, 0, m.PasteCount())
}
```

**Implement:**

- In `Update()`, check `msg.Paste` on `tea.KeyMsg`
- Apply collapse threshold logic
- Store in `pastes[]` or insert inline

### Phase 3: Submit expansion

**Test first:**

```go
func TestPaste_ExpandedOnSubmit(t *testing.T) {
    var submitted string
    m := testModel(func(input string) tea.Cmd {
        submitted = input
        return nil
    })
    m = applyPaste(m, "func main() {\n\tfmt.Println(\"hello\")\n}")
    m = applyKey(m, "enter")
    assert.Equal(t, "func main() {\n\tfmt.Println(\"hello\")\n}", submitted)
}

func TestPaste_MixedTextAndChips(t *testing.T) {
    var submitted string
    m := testModel(func(input string) tea.Cmd {
        submitted = input
        return nil
    })
    m = typeString(m, "review this: ")
    m = applyPaste(m, "line1\nline2\nline3")
    m = typeString(m, " thanks")
    m = applyKey(m, "enter")
    assert.Equal(t, "review this: line1\nline2\nline3 thanks", submitted)
}
```

**Implement:**

- Before calling `onSubmit`, expand chip markers using stored pastes
- Reset paste state after submit

### Phase 4: Chip rendering

**Test first:**

```go
func TestPaste_ChipRenderedWithStyle(t *testing.T) {
    m := testModel(nil)
    m = applyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30})
    m = applyPaste(m, "a\nb\nc\nd\ne")
    view := m.View()
    assert.Contains(t, view, "Pasted text #1")
}
```

**Implement:**

- In prompt rendering, detect chip markers via regex
- Apply `DimStyle` or a new `PasteChipStyle` to the marker text

### Phase 5: /clear resets paste state

**Test first:**

```go
func TestPaste_ClearResetsPastes(t *testing.T) {
    m := testModel(nil)
    m = applyPaste(m, "a\nb\nc")
    assert.Equal(t, 1, m.PasteCount())
    m = typeString(m, "/clear")  // need to clear input first
    // ... or use a direct clear
}
```

**Implement:** Reset `pastes` and `pasteCount` in `/clear` handler.

## Test Helper

```go
func applyPaste(m tui.Model, text string) tui.Model {
    msg := tea.KeyMsg{
        Type:  tea.KeyRunes,
        Runes: []rune(text),
        Paste: true,
    }
    updated, _ := m.Update(msg)
    return updated.(tui.Model)
}
```

## Files to Change

| File                          | Changes                                                                                    |
| ----------------------------- | ------------------------------------------------------------------------------------------ |
| `internal/tui/input.go`       | Add `InsertString()` method                                                                |
| `internal/tui/input_test.go`  | Tests for `InsertString()`                                                                 |
| `internal/tui/app.go`         | Add `pastes []PastedText`, paste detection in Update, expansion in submit, reset in /clear |
| `internal/tui/app_test.go`    | Paste collapse, inline, expansion, clear tests                                             |
| `internal/tui/prompt.go`      | Chip styling in render                                                                     |
| `internal/tui/prompt_test.go` | Chip render tests                                                                          |
| `internal/tui/styles.go`      | `PasteChipStyle` lipgloss style                                                            |

## Open Questions

1. **Should chips be atomic?** Claude Code treats them as opaque blocks. We could make the marker a single "unit" that gets deleted all-at-once (backspace on any part removes the whole chip). This is more complex but better UX. **Recommendation:** start with character-by-character deletion (simpler), add atomic deletion later.

2. **Preview shortcut?** Claude Code users heavily request this. Could add a shortcut (e.g., Ctrl+P when cursor is on a chip) to expand it inline temporarily. **Recommendation:** defer to a follow-up.

3. **Max paste size?** Should we cap stored paste content to prevent memory issues? **Recommendation:** cap at 1MB per paste, warn and truncate above that.
