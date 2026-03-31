# TODO: Task TUI Parity with Claude Code

## Context

Claude Code displays tasks in a persistent checklist panel with checkboxes.
AQL currently renders task tools as generic tool results (raw JSON in the transcript).

---

## Must Fix (parity gaps)

- [x] **Task panel widget** — Persistent panel that shows all tasks as a checklist, rendered below the current thinking/streaming indicator (not in the transcript). Panel updates in-place when tasks are created or updated.

- [x] **Checkbox rendering** — Render tasks with status icons:
  - `◻` pending
  - `◼` in_progress (or spinner)
  - `✓` completed (green checkmark)

- [x] **Suppress task tool output from transcript** — `task_create`, `task_update`, `task_list` should NOT render as tool call blocks in the chat transcript. They are internal bookkeeping, not user-facing tool results.

- [x] **Task state in TUI Model** — Add a `taskPanel taskState` field to the TUI `Model`. Update it when task tool events arrive. The panel reads from this state.

- [x] **Ctrl+T toggle** — Keybinding to show/hide the task panel. Show hint in status bar: `ctrl+t to hide/show tasks`.

- [x] **Display name mapping** — Add `task_create`, `task_update`, `task_list`, `agent`, `notebook_edit` to `toolDisplayNames` and `toolInputExtractors` in `transcript.go`.

## Nice to Have (beyond parity)

- [x] **`/tasks` slash command** — Show current task list on demand (useful after panel is hidden).

- [x] **Task progress counter** — Show `2/5 tasks completed` in panel header.

- [x] **Animate task transitions** — Brief highlight when a task status changes (flash green on completion, accent on create, fades after 2s).
