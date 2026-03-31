package diff

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Pure value types for representing git diffs: DiffStats,
//     DiffFile, DiffHunk, DiffLine, DiffLineType.
//
// MUST NOT GO HERE:
//   - Anything that imports other internal packages — diff has zero
//     internal dependencies.
//   - I/O, git execution, or parsing logic (those belong in run.go
//     and parse.go).
//   - TUI rendering or display concerns.
//
// Q: I need a new diff-related type. Where?
// A: If it's a pure data type with no dependencies, put it here.
//    If it's about parsing, put it in parse.go.
//    If it's about executing git, put it in run.go.
// ──────────────────────────────────────────────────────────────────

// DiffLineType identifies whether a diff line is context, added, or removed.
type DiffLineType int

const (
	DiffContext DiffLineType = iota
	DiffAdded
	DiffRemoved
)

// DiffLine represents a single line in a diff hunk.
type DiffLine struct {
	Type    DiffLineType
	Content string
}

// DiffHunk represents a contiguous block of changes in a file.
type DiffHunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []DiffLine
}

// DiffFile represents a single changed file with its diff hunks.
type DiffFile struct {
	Path         string
	LinesAdded   int
	LinesRemoved int
	IsBinary     bool
	Hunks        []DiffHunk
}

// DiffStats holds aggregate statistics across all changed files.
type DiffStats struct {
	FilesChanged int
	Additions    int
	Deletions    int
}
