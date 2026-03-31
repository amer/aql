# System Prompt: Named Parts, Logging, and Tool Description Removal

## What Changed

1. Refactored `BuildSystemPrompt` from a monolithic string concatenation into composable named parts (`PromptPart`), with structured logging of each part's name and size.
2. Removed duplicate tool descriptions from the system prompt — tools are already sent as structured API `tools` parameter.

## Why

A simple "hi" costs 3.4k input tokens. There was no visibility into what the system prompt contained or how much each section contributed. Tool descriptions were sent twice — as text in the system prompt AND as structured JSON schemas in the API `tools` parameter. Claude Code's reference implementation does not include tool descriptions in the system prompt text.

## How

- **`PromptPart{Name, Content}`** — new type in `internal/agent/` (agent-internal, not domain — it doesn't cross package boundaries)
- **`BuildPromptParts(cfg, claudeMD, workDir)`** — returns `[]PromptPart` with named sections: `role`, `system`, `environment`, `git` (optional), `project-context` (optional)
- **`JoinPromptParts(parts)`** — pure function, joins part contents with `\n\n`
- **`LogPromptParts(agentName, parts)`** — logs each part at Debug with name/chars, totals at Info
- **`BuildSystemPrompt`** — preserved as thin wrapper (`JoinPromptParts(BuildPromptParts(...))`)
- **Removed `ToolDescriptionsPrompt()`** — dead code after removing tools from system prompt parts

## Design Decision: PromptPart in agent/, not domain/

`PromptPart` is only used within the `agent` package for prompt assembly. Domain types (`Message`, `StreamEvent`, `ChatParams`) exist in `domain/` because they flow between packages. `PromptPart` doesn't cross a boundary, so it stays in `agent/` per architecture rule #1.

## Log Output (Debug level)

```
DBG prompt part agent=coder part=role chars=24
DBG prompt part agent=coder part=system chars=142
DBG prompt part agent=coder part=environment chars=203
DBG prompt part agent=coder part=git chars=89
DBG prompt part agent=coder part=project-context chars=312
INF system prompt assembled agent=coder parts=5 totalChars=770
```
