# Environment Validation

## Overview

Added startup validation for required environment variables with clear error messages.

## Changes

### CheckEnv (`internal/agent/env.go`)

Validates that `ANTHROPIC_API_KEY` is set and non-empty at startup. Returns an
actionable error message:

```text
ANTHROPIC_API_KEY is not set

  export ANTHROPIC_API_KEY=<your-key>
```

### Structured Logging (`log/slog`)

Added `slog` structured logging across all packages at I/O boundaries:

- `internal/agent/` — agent creation, API stream start/completion, errors
- `internal/events/` — subscribe/publish events
- `internal/memory/` — manager init, memory queries
- `internal/orchestrator/` — agent registration, workflow start, status changes

Log levels: `Debug` for decision points and internal state, `Info` for lifecycle
events, `Error` for failures.
