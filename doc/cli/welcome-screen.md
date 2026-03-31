# Welcome Screen — Spec

## Background

Claude Code displays an elaborate welcome screen on startup with a rounded border, ASCII mascot ("Clawd"), welcome greeting, project path, version, model info, and a right-side panel showing recent sessions and tips (on wide terminals). AQL currently shows a minimal 4-line header with box-drawing characters. This spec covers replacing the header with a proper welcome screen inspired by Claude Code.

## Goals

1. Replace the current `RenderHeader` with a bordered welcome box
2. Show welcome greeting, project path, model, and auth/billing info
3. Responsive layout: full box on wide terminals, compact on narrow
4. ASCII art logo for AQL identity
5. Show keyboard shortcut tips inside the welcome box

## Non-Goals

- Recent sessions panel (AQL doesn't have session persistence yet — defer)
- Animated mascot poses (defer to a future iteration)
- "What's new" / changelog feed (defer)
- Release notes integration

## Reference: Claude Code Behavior

### Layout Modes

Claude Code uses two layouts based on terminal width:

- **Horizontal** (width >= 70): Two-column layout inside a rounded border. Left column has greeting, logo, path info. Right column has feeds (recent sessions, tips, what's new).
- **Compact** (width < 70): Single-column layout. Greeting, logo, path — all centered, no right panel.

### Elements

| Element       | Description                                                                                         |
| ------------- | --------------------------------------------------------------------------------------------------- |
| Border        | Rounded (`borderStyle: "round"`) with brand color, title badge `" Claude Code vX.Y.Z "` at top-left |
| Greeting      | Bold text: `"Welcome back!"` or `"Welcome back {username}!"`                                        |
| Logo          | ASCII art mascot "Clawd" (block characters ▗▖▛▜▙▘▝█)                                                |
| Project path  | Dim text, truncated with `…` from left if too long                                                  |
| Billing/model | Dim text: `"{model} · {billing type}"` or `"{model} · {billing} · {org}"`                           |
| Right panel   | Two feed cards with title, bullet lines, footer link                                                |
| Separator     | Vertical single-line border between left and right columns                                          |

### Border Title

```
╭─ Claude Code v2.1.87 ────────────────────────╮
```

The title is positioned at top-left with offset, styled in brand color.

### Path Truncation

Long paths are truncated keeping first and last segments:

```
/Users/amer/…/some-project
```

## Design

### Part 1: Welcome Box Layout

Replace `RenderHeader` with `RenderWelcome` that draws a full bordered box.

#### Layout Modes

```go
type WelcomeLayout int

const (
    WelcomeHorizontal WelcomeLayout = iota // width >= 70
    WelcomeCompact                          // width < 70
)

func welcomeLayout(width int) WelcomeLayout
```

#### Data

```go
type WelcomeData struct {
    AppName     string // "AQL"
    Version     string // from build info or hardcoded
    ProjectPath string // working directory
    ModelName   string // current model
    Username    string // OS username (optional)
    Width       int    // terminal width
}
```

#### Signature

```go
func RenderWelcome(data WelcomeData) string
```

### Part 2: Greeting

```go
func welcomeGreeting(username string) string
```

- If username is non-empty and <= 20 chars: `"Welcome back {username}!"`
- Otherwise: `"Welcome back!"`
- Rendered bold

### Part 3: ASCII Logo

A simple, recognizable logo for AQL using block characters. Not a mascot — a geometric mark:

```
  ╭───╮
  │ ▶ │
  ╰───╯
```

Or a stylized "A" in block characters:

```
   ▄█▄
  █▀ ▀█
  █▄▄▄█
  █   █
```

The logo is rendered in the brand/accent color and placed between the greeting and the path info.

```go
func renderLogo() string
```

### Part 4: Path Truncation

```go
func truncatePath(path string, maxWidth int) string
```

Logic (matching Claude Code's `k18` function):

1. If path fits within maxWidth, return as-is
2. Split by `/`
3. Keep first segment and last segment
4. Fill middle segments from right until width is exceeded
5. Replace omitted middle with `…`

Example: `/Users/amer/Code/github.com/amer/aql` at width 30 → `/Users/amer/…/amer/aql`

### Part 5: Tips Panel (Horizontal Layout Only)

On wide terminals (>= 70 cols), show a right-side column with keyboard tips:

```go
func renderTips() string
```

Tips content:

```
  Tips
  ──────────────
  /model to switch models
  /diff to view changes
  /help for all commands
  Shift+↑↓ to scroll
```

Rendered in dim style with the title in accent color.

### Part 6: Full Render

#### Compact Layout (< 70 cols)

```
╭─ AQL v0.1.0 ─────────────────────╮
│                                   │
│         Welcome back amer!        │
│                                   │
│              ▄█▄                  │
│             █▀ ▀█                 │
│             █▄▄▄█                 │
│             █   █                 │
│                                   │
│    /Users/amer/…/amer/aql         │
│    claude-sonnet-4-20250514       │
│                                   │
╰───────────────────────────────────╯
```

- Rounded border in brand color
- Title badge at top-left: `" AQL vX.Y.Z "`
- Content centered
- Greeting bold, path and model dim

#### Horizontal Layout (>= 70 cols)

```
╭─ AQL v0.1.0 ──────────────────────────────────────────────────╮
│                                    │                           │
│         Welcome back amer!         │  Tips                     │
│                                    │  ──────────────           │
│              ▄█▄                   │  /model to switch models  │
│             █▀ ▀█                  │  /diff to view changes    │
│             █▄▄▄█                  │  /help for all commands   │
│             █   █                  │  Shift+↑↓ to scroll       │
│                                    │                           │
│    /Users/amer/…/amer/aql          │                           │
│    claude-sonnet-4-20250514        │                           │
│                                    │                           │
╰────────────────────────────────────────────────────────────────╯
```

- Two columns separated by `│`
- Left: greeting + logo + path (centered)
- Right: tips panel (left-aligned)

### Part 7: Styles

Add to `styles.go`:

```go
WelcomeBorderStyle  = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(accentColor)
WelcomeGreetStyle   = lipgloss.NewStyle().Bold(true).Foreground(textColor)
WelcomeLogoStyle    = lipgloss.NewStyle().Foreground(accentColor)
WelcomeDimStyle     = lipgloss.NewStyle().Foreground(dimColor)
WelcomeTipTitleStyle = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
WelcomeTipStyle     = lipgloss.NewStyle().Foreground(dimColor)
WelcomeBadgeStyle   = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
```

### Part 8: Integration

1. Replace `RenderHeader(...)` calls in `app.go` View() with `RenderWelcome(...)`
2. Remove `header.go` (or repurpose)
3. The welcome box is only shown at the top of the chat — once the user submits a prompt and gets a response, it scrolls up naturally like any other chat content

## Implementation Plan

### Phase 1: Path truncation + greeting (pure functions)

**Test first:**

```go
func TestTruncatePath_ShortPath(t *testing.T)
func TestTruncatePath_LongPath(t *testing.T)
func TestTruncatePath_RootPath(t *testing.T)
func TestTruncatePath_HomeDir(t *testing.T)
func TestTruncatePath_SingleSegment(t *testing.T)
func TestWelcomeGreeting_WithUsername(t *testing.T)
func TestWelcomeGreeting_EmptyUsername(t *testing.T)
func TestWelcomeGreeting_LongUsername(t *testing.T)
```

**Files:** `internal/tui/welcome.go`, `internal/tui/welcome_test.go`

### Phase 2: Logo rendering

**Test first:**

```go
func TestRenderLogo_NotEmpty(t *testing.T)
func TestRenderLogo_ConsistentWidth(t *testing.T)
```

**Files:** `internal/tui/welcome.go`

### Phase 3: Layout mode selection

**Test first:**

```go
func TestWelcomeLayout_Wide(t *testing.T)
func TestWelcomeLayout_Narrow(t *testing.T)
func TestWelcomeLayout_Threshold(t *testing.T)
```

**Files:** `internal/tui/welcome.go`

### Phase 4: Tips panel

**Test first:**

```go
func TestRenderTips_ContainsKeyboardShortcuts(t *testing.T)
func TestRenderTips_FitsWidth(t *testing.T)
```

**Files:** `internal/tui/welcome.go`

### Phase 5: Full RenderWelcome — compact mode

**Test first:**

```go
func TestRenderWelcome_Compact_ContainsGreeting(t *testing.T)
func TestRenderWelcome_Compact_ContainsPath(t *testing.T)
func TestRenderWelcome_Compact_ContainsModel(t *testing.T)
func TestRenderWelcome_Compact_ContainsBorder(t *testing.T)
func TestRenderWelcome_Compact_ContainsBadge(t *testing.T)
```

**Files:** `internal/tui/welcome.go`

### Phase 6: Full RenderWelcome — horizontal mode

**Test first:**

```go
func TestRenderWelcome_Horizontal_ContainsTips(t *testing.T)
func TestRenderWelcome_Horizontal_ContainsSeparator(t *testing.T)
func TestRenderWelcome_Horizontal_TwoColumns(t *testing.T)
```

**Files:** `internal/tui/welcome.go`

### Phase 7: Integration — replace RenderHeader

**Test first:**

```go
func TestView_ShowsWelcomeBox(t *testing.T) // update existing header test
```

**Files:** `internal/tui/app.go`, `internal/tui/app_test.go`, remove/update `internal/tui/header.go`

## Files to Change

| File                           | Changes                                                                                                |
| ------------------------------ | ------------------------------------------------------------------------------------------------------ |
| `internal/tui/welcome.go`      | New — WelcomeData, RenderWelcome, truncatePath, welcomeGreeting, renderLogo, renderTips, welcomeLayout |
| `internal/tui/welcome_test.go` | New — all welcome screen tests                                                                         |
| `internal/tui/styles.go`       | Add welcome-related styles                                                                             |
| `internal/tui/app.go`          | Replace `RenderHeader` calls with `RenderWelcome`                                                      |
| `internal/tui/header.go`       | Remove or deprecate                                                                                    |
| `internal/tui/header_test.go`  | Remove or update                                                                                       |

## Open Questions

1. **Logo design?** The "A" block character logo is a placeholder. Could also use a simpler geometric shape or text-only badge. **Recommendation:** start with the block "A", iterate based on how it looks in the terminal.

2. **Version string source?** Currently no version tracking in AQL. **Recommendation:** hardcode `"dev"` for now, add build-time injection later via `-ldflags`.

3. **Username source?** `os.User` or `$USER` env var. **Recommendation:** use `os.User` with fallback to empty (shows generic greeting).

4. **Welcome box height?** Fixed or dynamic? **Recommendation:** dynamic based on content, but with a minimum height of ~8 lines for visual consistency.
