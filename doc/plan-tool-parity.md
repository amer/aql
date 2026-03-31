# Tool Parity Implementation Plan

## Status Assessment

From `tool-parity-todo.md`, cross-referenced with the current codebase:

**Already implemented:** All 10 current tools (read_file, write_file, edit, list_directory, bash, glob, web_fetch, web_search, ask_user, grep). Also already done: dynamic tool descriptions, environment info, git status injection, date injection, CLAUDE.md hot-reload, auto-compaction, precise token counting.

**Still missing (this plan):**

| Tool                                | Priority | Size | Status   |
| ----------------------------------- | -------- | ---- | -------- |
| task_create, task_update, task_list | P1       | M    | Done     |
| agent                               | P1       | L    | Done     |
| notebook_edit                       | P2       | S    | Done     |
| lsp                                 | P2       | L    | Deferred |

LSP is deferred (requires running gopls, high complexity, low immediate value). The other three are planned below.

---

## Step 1: Task Tracking Tools (P1, Size M)

**Goal:** Let the agent break work into visible, tracked steps.

### 1a. Domain type — `internal/agent/tools/task.go`

Pure in-memory task store, no external dependencies.

```
Task struct: ID (int), Description (string), Status (pending|in_progress|completed)
TaskStore struct: tasks []Task, nextID int, mu sync.Mutex
Methods: Create(desc) -> Task, Update(id, status) -> error, List() -> []Task
```

Tests first (`task_test.go`):

- Create a task → ID assigned, status defaults to "pending"
- Update status → status changes
- Update nonexistent ID → error
- List → returns all tasks in order

### 1b. Tool definitions — add to `defs.go`

Three new entries in `Definitions()`:

- `task_create` — input: `{description: string}`, returns JSON `{id, description, status}`
- `task_update` — input: `{id: int, status: string}`, returns JSON `{id, description, status}`
- `task_list` — input: `{}`, returns JSON array of tasks

### 1c. Tool handlers — `task.go`

Three handler functions: `execTaskCreate`, `execTaskUpdate`, `execTaskList`.

The `TaskStore` must be shared across tool calls within a single agent session. Options:

- **Chosen approach:** `TaskStore` is created in `DefaultExecutor` and closed over by the handlers. This keeps it session-scoped and requires no changes to the `toolHandler` signature.

Register in `buildRegistry()`.

### 1d. System prompt update

Add task tools to the agent's system prompt instructions (the model needs to know _when_ to use them, not just that they exist — the dynamic tool descriptions handle the "what").

---

## Step 2: Sub-Agent Tool (P1, Size L)

**Goal:** Agent can spawn child agents with isolated conversation context.

### 2a. Agent factory interface

The `agent` tool needs to create child `Agent` instances. To avoid circular dependencies and keep tools testable:

- Define `AgentSpawner` interface in `tools` package:
  ```go
  type AgentSpawner interface {
      Spawn(ctx context.Context, prompt string) (string, error)
  }
  ```
- Real implementation lives in `agent` package, creates a child `Agent` with:
  - Same `ChatClient`, `Config` (fresh history, custom system prompt)
  - Recursion depth tracking (max 3 levels)
  - Runs `agent.Run()`, collects all text, returns concatenated result

### 2b. Tool definition — add to `defs.go`

- `agent` tool — input: `{prompt: string, description: string}`
- Description: "Spawn a sub-agent to handle a task independently. The sub-agent has its own conversation context and tool access."

### 2c. Tool handler — `agent_tool.go`

- `execAgent(ctx, input, spawner)` calls `spawner.Spawn(ctx, prompt)`
- Returns the sub-agent's text output as the tool result
- Timeout: inherit parent context (or add a 5-minute sub-agent timeout)

### 2d. Wire spawner into executor

- Add `WithAgentSpawner(AgentSpawner)` option pattern (similar to `AskUserFn`)
- `DefaultExecutor` accepts optional spawner, closes over it in the `agent` handler
- `main.go` creates the spawner and passes it through

### 2e. Recursion depth

- Add `depth int` to agent spawner or config
- Each child increments depth
- Reject spawning when depth >= 3

---

## Step 3: Notebook Edit Tool (P2, Size S)

**Goal:** Edit Jupyter notebook cells by parsing/modifying `.ipynb` JSON.

### 3a. Tool definition

- `notebook_edit` — input: `{path: string, cell_index: int, new_source: string, cell_type: string}`
- Reads `.ipynb`, parses JSON, replaces cell source, writes back

### 3b. Tool handler — `notebook.go`

- Parse notebook JSON structure (cells array with source/cell_type)
- Validate cell_index bounds
- Replace cell source (split new_source into lines array)
- Optionally change cell_type (code/markdown)
- Write back with same JSON formatting

---

## Implementation Order

1. **Task store + tests** (pure domain logic, no deps)
2. **Task tool defs + handlers + tests** (wire into registry)
3. **AgentSpawner interface + tests** (with fake spawner)
4. **Agent tool def + handler + tests**
5. **Wire real spawner in main.go**
6. **Notebook edit tool** (if time permits)

Each step: failing test → implement → green → refactor → commit.
