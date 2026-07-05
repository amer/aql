# glob: `**` matches any number of path segments

## What changed

`glob` now treats `**` as a wildcard for zero or more path segments anywhere in
the pattern, so `src/**/*.ts` matches `src/a.ts`, `src/nested/b.ts`, and
`src/nested/more/c.ts` alike.

## Why

Resolves **H7**.

The matcher delegated to `filepath.Match`, which has no `**` support — it reads
`**` as a single-segment `*`. Only a leading `**/` was special-cased (by
trimming it and matching the base name). So the documented `src/**/*.ts` example
matched exactly one intermediate directory level (`src/a/b.ts`) and returned
"No files matched" for direct children and deeper nesting.

## Design

- `matchesPattern` now calls `matchGlob`, which splits the pattern and the
  relative path on `/` and matches segment by segment in `matchSegments`.
- A `**` segment recurses over every possible span of path segments (including
  zero), giving true cross-directory matching regardless of where `**` appears.
- Non-`**` segments still use `filepath.Match`, preserving single-segment `*`
  and `?` semantics.
- The base-name fallback is retained but now scoped to patterns without a `/`,
  so a bare `*.ts` still finds files at any depth while `src/*.ts` stays
  anchored to one level.

## Tests

`TestGlob_DoublestarWithPrefixMatchesAllDepths` builds files at three depths
under `src/` plus one outside it, globs `src/**/*.ts`, and asserts all three
`src` files match while the outsider does not. The existing
`TestGlob_RecursiveDoublestar`, `TestGlob_SkipsHiddenDirs`, and non-recursive
glob tests remain green.
