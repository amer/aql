# AQL CLI Commands

## Startup

```bash
aql                        # Launch TUI (requires ANTHROPIC_API_KEY or OAuth login)
aql auth login --console   # OAuth login via Anthropic Console
```

## Slash Commands (inside TUI)

| Command    | Description                   |
| ---------- | ----------------------------- |
| `/help`    | Show available commands       |
| `/model`   | Open model picker             |
| `/clear`   | Clear chat history            |
| `/agents`  | List active agents            |
| `/status`  | Show workflow status          |
| `/compact` | Compact conversation context  |
| `/spinner` | Cycle spinner animation style |
| `/exit`    | Exit AQL                      |
| `/quit`    | Exit AQL                      |

## Model Selection

The `/model` command opens an interactive picker showing models probed from the API.
Models are cached for 1 hour. Use arrow keys to navigate, Enter to confirm, Esc to cancel.

The last entry allows typing a custom model ID for preview/unreleased models.

## Authentication

OAuth login via `aql auth login --console` enables access to all Claude models
(Opus, Sonnet, Haiku) through the Claude Code billing mechanism. Tokens are saved
to `.aql_tokens.json` in the current directory and expire after 8 hours.

Without OAuth, AQL falls back to the `ANTHROPIC_API_KEY` environment variable,
which may have limited model access depending on workspace configuration.
