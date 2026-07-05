# Memoize the glamour markdown renderer by width

## What changed

`RenderMarkdown` now fetches its glamour `TermRenderer` from a width-keyed
cache (`rendererCache`) instead of calling `glamour.NewTermRenderer` on every
invocation. The renderer is built once per terminal width and reused.

## Why

Resolves **H1** (the render-cost half).

`View()` runs on every frame (every 50–120 ms spinner tick). It rebuilds the
transcript and, for each assistant text part, called `RenderMarkdown`, which
constructed a brand-new glamour renderer — the expensive step, since it
compiles the whole style config. During a streaming response the transcript
grows and the number of text parts grows, so the per-frame cost climbed with
the response length: quadratic work for a linear stream.

Terminal width changes rarely, so a single renderer serves nearly every call.

## Design

- `buildRenderer(width)` isolates the costly construction; `rendererCache.get`
  memoizes it behind a mutex keyed by wrap width.
- The cache is a process-wide `var markdownCache`: a _pure_ memo — every width
  maps to a deterministic renderer, so sharing it adds no observable state, only
  removes rebuild cost. `View()` is single-goroutine, and the mutex guards the
  map regardless.
- `TestRendererCache_ReusesRendererPerWidth` (internal test) pins the contract:
  repeated calls at one width build once; a new width builds again.
- Renderer output is unchanged — the existing transcript rendering tests still
  pass.
