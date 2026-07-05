# Redact credential headers in the e2e recorder

## What changed

The recording proxy now redacts credential-bearing headers (`X-Api-Key`,
`Authorization`, `Proxy-Authorization`, `Cookie`, `Set-Cookie`) to `REDACTED`
before an exchange is stored. This happens in `Recorder.record`, the single
point every captured exchange passes through.

## Why

Resolves **C10**.

The recorder captured full request headers verbatim into
`test/e2e/testdata/*/exchanges.json`, which is committed to git. The only reason
the current fixtures don't contain a live key is that someone hand-edited them.
The next `E2E_RECORD=1` run would have written a real `X-Api-Key` straight into
a tracked file. Scrubbing at capture time removes the footgun entirely.

## Design

- `scrubHeaders(http.Header)` in `test/e2e/recorder.go` — canonicalizes each
  header name and blanks the value if it is in the `sensitiveHeaders` set.
- Called on both request and response headers inside `record`, so failed-request
  and streaming paths are covered by the same guard.
