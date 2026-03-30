# Claude Code TUI Features

Comprehensive reference of all TUI (Terminal User Interface) features in Claude Code, extracted from the [official repository](https://github.com/anthropics/claude-code) and documentation.

## 1. Keyboard Shortcuts — General Controls

| Shortcut                     | Description                                |
| ---------------------------- | ------------------------------------------ |
| `Ctrl+C`                     | Cancel current input or generation         |
| `Ctrl+D`                     | Exit session                               |
| `Ctrl+G` / `Ctrl+X Ctrl+E`   | Open prompt in external text editor        |
| `Ctrl+L`                     | Clear terminal screen (keeps conversation) |
| `Ctrl+O`                     | Toggle verbose transcript output           |
| `Ctrl+R`                     | Reverse search command history             |
| `Ctrl+V` / `Cmd+V` / `Alt+V` | Paste image from clipboard                 |
| `Ctrl+B`                     | Background running tasks                   |
| `Ctrl+T`                     | Toggle task list visibility                |
| `Ctrl+S`                     | Stash current prompt                       |
| `Ctrl+_`                     | Undo last action                           |
| `Esc+Esc`                    | Open rewind/summarize dialog               |
| `Shift+Tab` / `Alt+M`        | Cycle permission modes                     |
| `Alt+P` / `Cmd+P`            | Open model picker                          |
| `Alt+T` / `Cmd+T`            | Toggle extended thinking                   |
| `Alt+O` / `Meta+O`           | Toggle fast mode                           |
| `Up/Down`                    | Navigate command history                   |
| `Left/Right`                 | Cycle through dialog tabs                  |

## 2. Text Editing (Readline-style)

| Shortcut          | Description                  |
| ----------------- | ---------------------------- |
| `Ctrl+K`          | Delete to end of line        |
| `Ctrl+U`          | Delete to line start         |
| `Ctrl+Y`          | Paste deleted text (yank)    |
| `Alt+Y`           | Cycle kill ring history      |
| `Alt+B` / `Alt+F` | Move word backward/forward   |
| `Ctrl+J`          | Insert line feed (multiline) |

## 3. Multiline Input

- `\` + `Enter` — works in all terminals
- `Option+Enter` — macOS default
- `Shift+Enter` — works natively in iTerm2, WezTerm, Ghostty, Kitty; run `/terminal-setup` for others
- `Ctrl+J` — control sequence
- Paste mode — auto-detected for code blocks and logs

## 4. Quick Input Prefixes

| Prefix           | Description                                       |
| ---------------- | ------------------------------------------------- |
| `/`              | Opens command/skill menu with autocomplete        |
| `!`              | Bash mode — run shell commands directly           |
| `@`              | File/folder path autocomplete with fuzzy matching |
| `@terminal:name` | Reference terminal output (VS Code)               |
| `@browser`       | Browser tool invocation (VS Code)                 |

## 5. Slash Commands (TUI-triggering)

| Command                           | TUI Element                                          |
| --------------------------------- | ---------------------------------------------------- |
| `/help`                           | Help menu                                            |
| `/clear` (`/reset`, `/new`)       | Clear conversation                                   |
| `/compact [instructions]`         | Compact conversation with optional focus             |
| `/config` (`/settings`)           | Tabbed settings interface                            |
| `/context`                        | Colored grid context visualization                   |
| `/copy [N]`                       | Interactive code block picker (`w` to write to file) |
| `/color [color]`                  | Set prompt bar color                                 |
| `/cost`                           | Token usage statistics                               |
| `/diff`                           | Interactive diff viewer                              |
| `/doctor`                         | Diagnose installation/settings                       |
| `/effort [level]`                 | Set model effort level (low/medium/high/max/auto)    |
| `/export [filename]`              | Export conversation (clipboard or file)              |
| `/fast [on\|off]`                 | Toggle fast mode                                     |
| `/branch` (`/fork`)               | Fork conversation                                    |
| `/keybindings`                    | Open keybindings config                              |
| `/mcp`                            | MCP server management dialog                         |
| `/memory`                         | Edit CLAUDE.md memory files                          |
| `/model [model]`                  | Model picker with effort adjustment                  |
| `/permissions` (`/allowed-tools`) | Tabbed permissions viewer                            |
| `/plan [description]`             | Enter plan mode                                      |
| `/plugin`                         | Plugin management dialog                             |
| `/rename [name]`                  | Rename session                                       |
| `/resume` (`/continue`)           | Session picker                                       |
| `/rewind` (`/checkpoint`)         | Rewind menu with scrollable messages                 |
| `/sandbox`                        | Sandbox toggle with tabbed interface                 |
| `/skills`                         | List available skills                                |
| `/stats`                          | Usage visualization, streaks, history                |
| `/status`                         | Status settings (works while Claude is responding)   |
| `/statusline`                     | Configure custom status bar                          |
| `/terminal-setup`                 | Configure terminal keybindings                       |
| `/theme`                          | Theme picker                                         |
| `/vim`                            | Toggle vim editing mode                              |
| `/voice`                          | Toggle push-to-talk voice dictation                  |
| `/btw <question>`                 | Side question overlay (ephemeral)                    |
| `/remote-control` (`/rc`)         | Enable remote control from claude.ai                 |
| `/desktop` (`/app`)               | Hand off session to Desktop app                      |
| `/mobile` (`/ios`, `/android`)    | Show QR code for mobile app download                 |
| `/pr-comments [PR]`               | Fetch and display GitHub PR comments                 |
| `/release-notes`                  | View full changelog                                  |
| `/schedule`                       | Cloud scheduled task setup                           |
| `/security-review`                | Analyze pending changes for vulnerabilities          |
| `/stickers`                       | Order Claude Code stickers                           |
| `/tasks`                          | List and manage background tasks                     |
| `/usage`                          | Show plan usage limits and rate limit status         |
| `/insights`                       | Generate usage analysis report                       |

## 6. Bundled Skills

| Skill                       | Description                                              |
| --------------------------- | -------------------------------------------------------- |
| `/batch <instruction>`      | Orchestrate large-scale parallel changes across codebase |
| `/claude-api`               | Load Claude API reference material                       |
| `/debug [description]`      | Enable debug logging and troubleshoot issues             |
| `/loop [interval] <prompt>` | Run a prompt repeatedly on an interval                   |
| `/simplify [focus]`         | Review changed files for code reuse, quality, efficiency |

## 7. UI Components

### Prompt Bar / Input Area

- Text input with cursor positioning
- Customizable prompt bar color (`/color`)
- Session name display (`/rename`)
- Permission mode indicator (cycles with `Shift+Tab`)
- Context usage percentage indicator
- Fast mode `↯` icon (gray during cooldown)
- Vim mode indicator (NORMAL/INSERT)
- `hold Space to speak` hint (voice mode)
- Grayed-out prompt suggestions based on git/conversation history
- `[Image #N]` chips for pasted images
- `@` file mention autocomplete with fuzzy matching
- `!` bash mode with tab-completable history suggestions

### Status Line (Bottom Bar)

- Fully customizable via shell scripts (`/statusline`)
- Receives JSON session data (model, context, cost, git, vim mode, agent, worktree, rate limits)
- ANSI colors, multi-line output, OSC 8 clickable links
- Configurable padding
- Auto-updates after each assistant message, permission mode change, or vim mode toggle
- Debounced at 300ms
- Hides during autocomplete suggestions, help menu, and permission prompts

### PR Review Status (Footer)

- Clickable PR link (e.g., "PR #446")
- Color-coded underline: green (approved), yellow (pending), red (changes requested), gray (draft), purple (merged)
- Auto-updates every 60 seconds
- `Cmd+click` / `Ctrl+click` to open in browser

### Task List

- Toggle with `Ctrl+T`
- Shows up to 10 tasks with status indicators (pending, in progress, complete)
- Persists across context compactions
- Shareable across sessions via `CLAUDE_CODE_TASK_LIST_ID`

### Footer Navigation

- Footer items for tasks, teams, diff
- Arrow key navigation (left/right/up/down)
- Enter to open selected item
- Escape to clear selection

### Notification Area

- System notifications on the right side of status line row
- MCP server errors, auto-updates, token warnings
- Verbose mode token counter
- Desktop notifications via iTerm2/Kitty/Ghostty (tmux passthrough supported)
- Background task stuck notification (~45 seconds)
- Idle-return prompt after 75+ minutes

## 8. Interactive Dialogs and Pickers

### Model Picker

- Opened with `Alt+P` / `Cmd+P` / `/model`
- Left/right arrows to adjust effort level
- Custom model ID support via `ANTHROPIC_CUSTOM_MODEL_OPTION`
- Takes effect immediately without waiting for current response

### Theme Picker

- Opened with `/theme`
- Light and dark variants
- Colorblind-accessible (daltonized) themes
- ANSI themes using terminal color palette
- `Ctrl+T` to toggle syntax highlighting within picker

### Permission/Confirmation Dialogs

- Yes/No options (Y/Enter to confirm, N/Escape to decline)
- Up/Down to navigate options
- Tab for next field
- `Shift+Tab` to cycle permission modes from within dialog
- `Ctrl+E` to toggle permission explanation
- `Ctrl+D` to toggle permission debug info

### Rewind/Checkpoint Dialog

- Opened with `Esc+Esc` or `/rewind`
- Scrollable message list from session
- Actions: Restore code and conversation, Restore conversation, Restore code, Summarize from here, Never mind
- Vim-style navigation (J/K, Ctrl+P/Ctrl+N)
- Shift+J/Shift+K to jump to top/bottom

### Diff Viewer

- Opened with `/diff`
- Left/right arrows for diff source switching (current git diff vs individual Claude turns)
- Up/down for file navigation
- Enter to view details
- Escape to dismiss

### Session Picker (Resume)

- Search by keyword
- Browse by time (Today, Yesterday, Last 7 days)
- Rename and remove actions on hover
- Local and Remote tabs (VS Code)

### Copy Picker

- Interactive picker when code blocks present
- Select individual blocks or full response
- `w` key to write to file instead of clipboard

### Plugin Dialog

- Browse, discover, manage plugins
- Space to toggle plugin selection
- I to install selected plugins
- Scope selection: user/project/local

### Settings Interface (`/config`)

- Tabbed interface
- Search mode with `/`
- Status tab, theme, model, output style options
- "Show turn duration" toggle
- R to retry loading usage data on error

### Sandbox Dialog (`/sandbox`)

- Tab and arrow key navigation between tabs
- Dependencies tab (platform-specific)

## 9. Transcript Viewer

- Toggle with `Ctrl+O`
- `Ctrl+E` to toggle show all content
- `q`, `Ctrl+C`, `Esc` to exit
- `/` to search within transcript (press `n`/`N` to step through matches)
- `Ctrl+U`/`Ctrl+D` for half-page scroll
- Collapsed MCP read/search calls expand with `Ctrl+O`

## 10. Vim Editor Mode

Full vim-style editing toggled with `/vim` or configured via `/config`.

### Mode Switching

- `Esc` — NORMAL mode
- `i`/`I`/`a`/`A`/`o`/`O` — INSERT mode

### Navigation (NORMAL)

- `h`/`j`/`k`/`l` — character/line movement
- `w`/`e`/`b` — word movement
- `0`/`$`/`^` — line start/end
- `gg`/`G` — document start/end
- `f`/`F`/`t`/`T` with `;`/`,` repeat — find character

### Editing (NORMAL)

- `x` — delete character
- `dd`/`D` — delete line / to end of line
- `dw`/`de`/`db` — delete word
- `cc`/`C` — change line / to end of line
- `cw`/`ce`/`cb` — change word
- `yy`/`Y` — yank line
- `yw`/`ye`/`yb` — yank word
- `p`/`P` — paste after/before
- `>>`/`<<` — indent/dedent
- `J` — join lines
- `.` — repeat last command

### Text Objects

- `iw`/`aw` — inner/around word
- `iW`/`aW` — inner/around WORD
- `i"`/`a"`, `i'`/`a'` — inner/around quotes
- `i(`/`a(`, `i[`/`a[`, `i{`/`a{` — inner/around brackets

## 11. Visual Features and Rendering

### Markdown and Code

- Syntax highlighting for code blocks (toggleable via `Ctrl+T` in theme picker)
- Markdown rendering of responses
- Response text streams line-by-line as generated

### Diff Display

- Side-by-side diff comparisons
- Per-turn diffs and git diff views
- Inline diffs in VS Code extension

### Visual Indicators

- Context window usage percentage
- Token counts displayed as "1.5m" for >= 1M tokens
- Permission mode in status bar
- Fast mode `↯` icon (turns gray during cooldown)
- Colored prompt bar (customizable)
- `[Image #N]` chips for pasted images
- Issue/PR references as clickable links (`owner/repo#123`)
- Memory filenames highlight on hover, open on click
- Blue dot on VS Code tab (permission pending), orange dot (task finished while hidden)
- Waveform display during voice recording
- "keep holding..." during voice warmup

## 12. Themes and Output Styles

### Themes

- Light and dark variants
- Colorblind-accessible (daltonized) themes
- ANSI themes using terminal's native color palette
- Configurable via `/theme` or `/config`

### Output Styles

- **Default** — software engineering focused
- **Explanatory** — educational insights between tasks
- **Learning** — collaborative, `TODO(human)` markers
- **Custom** — via markdown files with frontmatter

## 13. Voice Dictation

- Toggle with `/voice`
- Hold `Space` (default, rebindable) for push-to-talk
- Live waveform display during recording
- Dimmed text during live transcription, solidifies on finalize
- Transcript inserts at cursor position
- 20+ languages supported
- Coding-vocabulary-tuned transcription
- Git branch/project name used as recognition hints
- Configurable push-to-talk key via `keybindings.json`
- Modifier combos (e.g., `meta+k`) skip warmup delay

## 14. Command History and Search

- Per-directory input history
- Up/Down arrows for navigation
- `Ctrl+R` for reverse interactive search
- Highlighted search terms in matches
- `Ctrl+R` cycles through older matches
- Tab/Esc to accept and edit; Enter to accept and execute
- `Ctrl+C` to cancel search
- History resets on `/clear`
- `!` bash mode history-based autocomplete (Tab to complete)

## 15. Autocomplete System

- Tab to accept suggestion
- Escape to dismiss
- Up/Down for previous/next suggestion
- Slash command autocomplete (type `/` then filter)
- `@` file path autocomplete with fuzzy matching
- `!` bash command history autocomplete
- Ghost-text suggestions (including just-submitted commands)
- Prompt suggestions after responses (Tab to accept, Enter to accept and submit)

## 16. Background Task Management

- `Ctrl+B` to background running tasks
- Background task IDs for tracking
- Output written to files (readable via Read tool)
- Left arrow closes from list view in background tasks panel
- Auto-cleanup on exit
- Auto-termination if output exceeds 5GB
- Stuck task notification after ~45 seconds
- `Ctrl+X Ctrl+K` to kill all background agents (press twice within 3 seconds)

## 17. Checkpointing / Rewind

- Automatic checkpoint at every user prompt
- Persists across sessions (30-day retention, configurable)
- `Esc+Esc` or `/rewind` to open rewind menu
- Five actions: Restore code+conversation, Restore conversation, Restore code, Summarize from here, Never mind
- Original prompt restored into input field after restore/summarize
- VS Code: hover over message for rewind button with fork/rewind/fork+rewind options

## 18. Customizable Keybindings

- Configuration file: `~/.claude/keybindings.json`
- Hot-reload (changes auto-detected without restart)
- 17 binding contexts: Global, Chat, Autocomplete, Settings, Confirmation, Tabs, Help, Transcript, HistorySearch, Task, ThemePicker, Attachments, Footer, MessageSelector, DiffDialog, ModelPicker, Select, Plugin
- Chord bindings (e.g., `ctrl+k ctrl+s`)
- Modifier keys: ctrl, alt/opt/option, shift, meta/cmd/command
- Uppercase letters imply Shift
- Unbind with `null`
- Reserved shortcuts: Ctrl+C, Ctrl+D, Ctrl+M
- Validation with `/doctor` for parse errors, conflicts, duplicates

## 19. Permission Modes (UI States)

| Mode                | Behavior                                      |
| ------------------- | --------------------------------------------- |
| `default`           | Prompts for file edits and commands           |
| `acceptEdits`       | Auto-accepts file edits, prompts for commands |
| `plan`              | Read-only exploration, plan file output       |
| `auto`              | Background classifier, minimal prompts        |
| `dontAsk`           | Auto-denies everything not pre-approved       |
| `bypassPermissions` | No prompts or checks                          |

Cycle with `Shift+Tab`; mode indicator visible in status bar.

## 20. Image Support

- Paste images from clipboard (`Ctrl+V` / `Cmd+V` / `Alt+V`)
- `[Image #N]` chips inserted at cursor for positional reference
- Attachment navigation: Right/Left arrows, Backspace/Delete to remove, Down/Escape to exit
- VS Code: Shift+drag files, click X to remove attachments

## 21. Context Window Management

- Context indicator showing usage percentage
- Token counts >= 1M displayed as "1.5m"
- `/context` for colored grid visualization with optimization suggestions
- Auto-compaction when context window fills
- `/compact` for manual compaction with optional focus instructions
- Summarize from rewind menu for targeted compaction

## 22. Side Questions (`/btw`)

- Ephemeral overlay that does not enter conversation history
- Available while Claude is processing a response
- Full visibility into current conversation context
- No tool access (answer only from existing context)
- Single response (no follow-up turns)
- Dismiss with Space, Enter, or Escape
- Low cost (reuses prompt cache)

## 23. Prompt Suggestions

- Grayed-out suggestion appears in prompt input
- Based on git history (first turn) and conversation history (subsequent turns)
- Tab to accept, Enter to accept and submit
- Start typing to dismiss
- Background request reusing prompt cache
- Disableable via `CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false`

## 24. Terminal Configuration

- `/terminal-setup` for Shift+Enter binding in VS Code, Alacritty, Zed, Warp
- Option/Alt as Meta key configuration (iTerm2, Terminal.app, VS Code)
- Terminal notification support (iTerm2, Kitty, Ghostty)
- tmux passthrough for notifications and progress bar (`set -g allow-passthrough on`)
- Terminal progress bar
- Terminal tab title auto-updates with session description (disableable with `CLAUDE_CODE_DISABLE_TERMINAL_TITLE`)
- Deep links (`claude-cli://`) open in preferred terminal
- IME composition support (CJK input renders inline)
- Screen reader cursor tracking
- Kitty keyboard protocol support

## 25. VS Code Extension-Specific Features

- Spark icon in Editor Toolbar, Activity Bar, Status Bar
- Graphical chat panel or terminal mode (`useTerminal` setting)
- Command Palette integration (`Cmd+Shift+P` / `Ctrl+Shift+P`)
- `Cmd+Esc` / `Ctrl+Esc` to toggle focus between editor and Claude
- `Cmd+Shift+Esc` / `Ctrl+Shift+Esc` to open new tab
- `Cmd+N` / `Ctrl+N` for new conversation (when Claude focused)
- `Option+K` / `Alt+K` to insert @-mention reference
- Inline diff viewer with accept/reject
- Plan mode opens plan as full markdown document with inline comments
- Multiple conversation tabs with colored status dots
- Permission mode selector at bottom of prompt box
- `/` command menu in prompt box
- Context indicator in prompt box
- Shift+Enter for multi-line in prompt box
- Session history dropdown with search and time grouping
- Plugin management UI (`/plugins`)
- Rewind picker (`Esc+Esc` or `/rewind`)
- Rate limit warning banner
- URI handler: `vscode://anthropic.claude-code/open` with prompt and session parameters
- Draggable panel positioning (sidebar, editor area, secondary sidebar)
- Learn Claude Code onboarding checklist
- Selection indicator in prompt footer (eye/eye-slash toggle)
- Shift+drag files into prompt box as attachments

## 26. Desktop App-Specific Features

- Visual diff review
- Multiple sessions side-by-side
- Schedule recurring tasks
- Permission mode selector next to send button
- Session management from Dispatch
