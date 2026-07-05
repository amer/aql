# Hermetic auth resolution, no working-dir credential search

## What changed

`auth.ResolveAPIKey` no longer takes a `workDir` and no longer searches the
working directory for saved tokens. Token discovery now runs over an explicit,
injectable list of directories:

- `ResolveAPIKey()` — production entry point; composes `defaultTokenSearchDirs()`
  (OS user-config dir, then home dir).
- `ResolveAPIKeyFromDirs(dirs []string)` — core logic with dirs injected so
  tests stay hermetic.

## Why

Two audit findings, one fix:

- **C8** — the old `tokenSearchDirs` put `workDir` first, so a hostile repo
  could commit a token file and silently hijack the session. The working
  directory is now excluded entirely.
- **C11** — because resolution unconditionally read the real user-config and
  home dirs, the `TestResolveAPIKey_NoTokens*` tests read the developer's live
  OAuth tokens on a logged-in machine, failed, and leaked a real API key into
  pre-commit output — blocking every commit. Tests now pass a `t.TempDir()` via
  `ResolveAPIKeyFromDirs` and never touch real credentials.
