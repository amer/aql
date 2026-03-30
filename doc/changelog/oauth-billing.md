# OAuth Authentication and Billing Header

## Summary

Added OAuth PKCE login flow and Claude Code billing header mechanism to unlock
Opus/Sonnet access for Console users whose workspace only has Haiku enabled.

## Changes

- **OAuth login**: `aql auth login --console` runs a full OAuth PKCE flow with
  local callback server, token exchange, and API key creation
- **Billing header**: OAuth agents inject `x-anthropic-billing-header` in the
  system prompt, plus adaptive thinking and beta headers — this routes billing
  through the Claude Code subscription instead of workspace limits
- **Dynamic model list**: Models are probed from the API with billing-aware
  requests instead of being hardcoded. Results are cached for 1 hour.
- **Background startup**: Model probing runs in a background goroutine. The TUI
  starts instantly using cached models, with a "Bootstrapping..." spinner while
  the probe completes.

## Key Discovery

Claude Code accesses Opus/Sonnet via a billing header in the system prompt, not
through a special API endpoint or different authentication. See ADR-003 for the
full reverse-engineering findings.

## Testing

- Unit tests for billing header injection and cache save/load
- Integration tests for OAuth key + Opus access (gated behind `AQL_LIVE_TEST=1`)
- Deterministic fake HTTP servers for all non-live tests
