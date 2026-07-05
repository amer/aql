# web_fetch SSRF guard: block private and link-local hosts

## What changed

`web_fetch` now resolves the target host and refuses any URL that points at a
loopback, private, link-local, unspecified, or multicast address, and rejects
non-HTTP(S) schemes. The guard is injectable via `WithFetchGuard`.

## Why

Resolves **H5**.

The fetch URL is untrusted agent input. With no restrictions the agent could
read the cloud metadata endpoint (`169.254.169.254`), `localhost` services, and
intranet hosts — a server-side request forgery vector.

## Design

- `FetchGuard func(*url.URL) string` returns a refusal reason ("" = allow),
  matching the tool convention that failures are human-readable strings.
- The default `blockPrivateNetworks` guard checks the scheme, resolves the host
  with `net.LookupIP`, and blocks if any resolved IP is private. The
  classification lives in the pure `isPrivateIP`, which composes the stdlib
  `net.IP` predicates (`IsLoopback`, `IsPrivate`, `IsLinkLocalUnicast`, …) so
  `169.254.169.254` and `fe80::/10` are covered.
- `NewExecutor` installs `blockPrivateNetworks` by default, so the production
  wiring in `main.go` is protected with no change. Only `web_fetch` is guarded —
  `web_search`'s host is ours, not user-controlled.
- `WithFetchGuard` lets tests fetch httptest servers on `127.0.0.1` by injecting
  a permissive guard; `execTool` uses it.
- `TestWebFetch_BlocksLinkLocalMetadataAddress` drives the default (secure)
  executor against `169.254.169.254` and asserts the refusal.

## Known limitation

The guard resolves the host once before dialing, so a DNS-rebinding attacker
that returns a public IP at check time and a private IP at connect time is not
fully defeated. Closing that TOCTOU window requires a custom dialer that
re-validates the socket address; it is out of scope for this fix, which closes
the direct-address and static-hostname vectors the finding named.
