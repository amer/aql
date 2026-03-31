# Model Configuration

## Overview

Made the Claude model configurable per agent instead of hardcoding a single model.

## Changes

### ResolveModel (`internal/agent/model.go`)

Resolves shortcut names to full model identifiers:

| Shortcut       | Model ID             |
| -------------- | -------------------- |
| `""` (default) | `claude-haiku-4-5`   |
| `haiku`        | `claude-haiku-4-5`   |
| `sonnet`       | `claude-sonnet-4-5`  |
| `opus`         | `claude-opus-4-5`    |
| anything else  | passed through as-is |

Default is Haiku — works with all API key tiers.

### Config.Model Field

`agent.Config` now has a `Model` field. The runner calls `ResolveModel(a.config.Model)`
instead of hardcoding the model.

### TUI Model Display

The resolved model name is shown in the header and status bar.
