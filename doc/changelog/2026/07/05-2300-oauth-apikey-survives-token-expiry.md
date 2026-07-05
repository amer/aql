# OAuth API key survives access-token expiry; drop dead refresh code

## What changed

- `ResolveAPIKeyFromDirs` no longer falls back to `ANTHROPIC_API_KEY` when the
  stored OAuth **access token** has expired. The API key minted at login is a
  separate, long-lived credential; expiry of the short-lived access token now
  logs a debug line and the minted key is used unchanged. The only remaining
  reason to skip OAuth is an empty `APIKey`.
- Removed dead token-refresh code with zero production callers:
  `RefreshAccessToken`, `Tokens.NeedsRefresh`, and the `refreshThreshold`
  constant, plus their tests.

## Why

Resolves **C7**. `ResolveAPIKey` treated access-token expiry as a reason to
abandon OAuth entirely, discarding a still-valid API key and silently switching
billing to whatever `ANTHROPIC_API_KEY` happened to be set — or failing outright
if it was unset. The refresh path that would have justified the expiry check was
never wired up (dead code), so the check had no upside and a real downside.
