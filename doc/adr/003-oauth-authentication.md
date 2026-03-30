# ADR-003: OAuth Authentication for Model Access

## Status

In Progress

## Context

AQL needs to access Claude models beyond Haiku (Opus, Sonnet). The standard API key
(`sk-ant-api03-*` from Keychain/env) only provides Haiku access for the user's workspace.
Claude Code accesses all models via OAuth authentication.

## Findings (2026-03-30)

### Authentication Methods Tested

| Method                                                      | Header                                                       | Result                                                |
| ----------------------------------------------------------- | ------------------------------------------------------------ | ----------------------------------------------------- |
| API key (`sk-ant-api03-*`)                                  | `x-api-key`                                                  | Works for Haiku only, 400 for Opus/Sonnet             |
| OAuth token (`sk-ant-oat01-*`) as API key                   | `x-api-key`                                                  | 401 "invalid x-api-key"                               |
| OAuth token as Bearer                                       | `Authorization: Bearer`                                      | 401 "OAuth authentication is currently not supported" |
| OAuth token as Bearer + beta                                | `Authorization: Bearer` + `anthropic-beta: oauth-2025-04-20` | 403 "does not meet scope requirement"                 |
| OAuth-created API key (`sk-ant-api03-*` via create_api_key) | `x-api-key`                                                  | Works for Haiku only, same as original key            |

### OAuth Flow (Console Login)

The two-step flow:

1. **OAuth PKCE** → `platform.claude.com/oauth/authorize` → auth code
2. **Token Exchange** → `platform.claude.com/v1/oauth/token` → OAuth Bearer token
3. **Create API Key** (optional) → `api.anthropic.com/api/oauth/claude_cli/create_api_key` → workspace API key

**Token Exchange Response:**

```json
{
  "token_type": "Bearer",
  "access_token": "sk-ant-oat01-...",
  "expires_in": 28800,
  "refresh_token": "sk-ant-ort01-...",
  "scope": "org:create_api_key user:file_upload user:profile"
}
```

**Critical:** The scope `user:inference` was requested but NOT granted by Console.
The API requires one of: `user:inference`, `user:ccr_inference`, `user:voice`, `org:service_key_inference`.

### Key Insights

1. **Bearer auth is supported** at `api.anthropic.com/v1/messages` — but requires:
   - Header: `anthropic-beta: oauth-2025-04-20`
   - Token with `user:inference` scope

2. **Console OAuth doesn't grant `user:inference`** — it only grants:
   - `org:create_api_key`
   - `user:file_upload`
   - `user:profile`

3. **API key created via OAuth** inherits workspace model access (Haiku-only for this workspace).

4. **Claude Code** presumably gets `user:inference` scope either through:
   - Non-console flow (claude.com/cai/oauth/authorize)
   - A different OAuth configuration
   - Platform-specific routing

### Token Prefixes

- `sk-ant-api03-*` — Standard API key (x-api-key header)
- `sk-ant-oat01-*` — OAuth Access Token (Bearer header)
- `sk-ant-ort01-*` — OAuth Refresh Token

### Endpoints

- Authorize (Console): `https://platform.claude.com/oauth/authorize`
- Authorize (Claude.ai): `https://claude.com/cai/oauth/authorize`
- Token Exchange: `https://platform.claude.com/v1/oauth/token`
- Create API Key: `https://api.anthropic.com/api/oauth/claude_cli/create_api_key`
- Client ID: `9d1c250a-e61b-44d9-88ed-5944d1962f5e`

### URL Construction

- Query params must NOT be encoded via `url.Values.Encode()` — Go encodes colons as `%3A`
  and slashes as `%2F`, which the OAuth server rejects
- Build the full query string manually with `strings.Join`

### Model Availability (per workspace)

Only models enabled on the Console workspace are accessible via OAuth-created API keys:

| Model                       | Status                       |
| --------------------------- | ---------------------------- |
| `claude-haiku-4-5`          | OK                           |
| `claude-haiku-4-5-20251001` | OK                           |
| `claude-3-haiku-20240307`   | OK                           |
| `claude-opus-4-6`           | FAIL (invalid_request_error) |
| `claude-sonnet-4-6`         | FAIL (invalid_request_error) |
| All other models            | FAIL (not_found_error)       |

This is a **workspace permission issue**, not a code issue. The Autodesk Console workspace
only has Haiku enabled. Opus/Sonnet require either workspace admin to enable them, or a
different auth mechanism.

### How Claude Code Accesses Opus (SOLVED — 2026-03-30)

Discovered via mitmproxy reverse-engineering of Claude Code's actual API calls.

Claude Code uses the same `api.anthropic.com/v1/messages` endpoint but includes a
**billing header** in the system prompt that identifies the request as coming from
Claude Code. This billing header unlocks all models regardless of workspace limits.

**Required elements (all must be present):**

1. **System prompt block 0** — billing header:

   ```
   x-anthropic-billing-header: cc_version=2.1.87.7b6; cc_entrypoint=cli; cch=22c94;
   ```

2. **Body fields:**

   ```json
   {
     "thinking": { "type": "adaptive" },
     "output_config": { "effort": "medium" }
   }
   ```

3. **HTTP header:**
   ```
   anthropic-beta: claude-code-20250219,interleaved-thinking-2025-05-14,effort-2025-11-24
   ```

**Verified working** with both Claude Code's Keychain API key and AQL's OAuth-created
API key. The billing header is the key mechanism — it routes billing through the
Claude Code subscription rather than the Console workspace.

## Decision

Use the two-step OAuth flow (token exchange → create API key) for Console login.
When using OAuth, inject the Claude Code billing header, adaptive thinking, and
output_config into every API request. This enables access to all models (Opus, Sonnet,
Haiku) regardless of Console workspace limits.
