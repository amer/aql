# Tool Parity TODO

Tools AQL needs to match Claude Code's agent capabilities.

## Current tools (implemented)

- [x] `read_file` — read file contents
- [x] `write_file` — create/overwrite files
- [x] `list_directory` — list directory entries
- [x] `bash` — execute shell commands
- [x] `grep` — regex search in files
- [x] `web_fetch` — fetch URL contents, extract text from HTML
- [x] `web_search` — search the web via DuckDuckGo
- [x] `ask_user` — ask the user a clarifying question, pause until they respond
- [x] `edit` — targeted find/replace edits (unique match or replace_all)
- [x] `glob` — file pattern matching with `**` support, sorted by mod time

## Missing tools

### P1 — Important for complex workflows

- [ ] **agent** — Spawn sub-agents for parallel/independent tasks
  - Input: `prompt`, `description`
  - Runs a child agent with its own conversation context
  - Returns the sub-agent's final result
  - Enables parallel research, search, and code exploration

- [ ] **task** — Track progress on multi-step work
  - Sub-tools: `task_create`, `task_update`, `task_list`
  - Input (create): `description`, `status`
  - Input (update): `id`, `status`, `result`
  - Lets the agent break work into discrete tracked steps
  - Useful for user visibility into long-running operations

### P2 — Nice to have

- [ ] **notebook_edit** — Edit Jupyter notebook cells
  - Input: `path`, `cell_index`, `new_source`, `cell_type`
  - Parse and modify `.ipynb` JSON structure
  - Low priority unless notebook workflows become common

- [ ] **lsp** — Language server protocol queries
  - Sub-tools: `go_to_definition`, `find_references`, `diagnostics`
  - Input: `file_path`, `line`, `column`
  - Requires running an LSP server (gopls for Go)
  - High complexity, but enables precise code navigation

### Not in scope (Claude Code infra features, not agent tools)

- **Plan mode** — enter/exit planning mode (UX feature, not a tool)
- **Worktree** — git worktree isolation for sub-agents (part of agent infra)
- **Cron** — recurring scheduled tasks (CLI feature)
- **MCP resources** — Model Context Protocol resource reading (plugin system)
- **Skill** — invoke slash-command skills (CLI dispatch, not an agent tool)

## Implementation notes

- Follow TDD: write failing tests first, then implement
- Each tool gets a `ToolDef` in `ToolDefinitions()` and a `case` in `executeTool()`
- Tool execution errors are returned as content strings, not Go errors
- Add tests using `httptest` or temp dirs as appropriate
- Update the system prompt to describe new tools to the agent
