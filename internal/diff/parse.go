package diff

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseNumstat parses the output of `git diff HEAD --numstat` into
// a slice of DiffFile (with path and line counts) and aggregate DiffStats.
// Binary files show as "-\t-\tpath" and get IsBinary=true with zero counts.
func ParseNumstat(output string) ([]DiffFile, DiffStats) {
	if strings.TrimSpace(output) == "" {
		return nil, DiffStats{}
	}

	var files []DiffFile
	var stats DiffStats

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}

		path := parts[2]

		// Binary files have "-" for both added and removed counts.
		if parts[0] == "-" && parts[1] == "-" {
			files = append(files, DiffFile{Path: path, IsBinary: true})
			stats.FilesChanged++
			continue
		}

		added, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		removed, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}

		files = append(files, DiffFile{
			Path:         path,
			LinesAdded:   added,
			LinesRemoved: removed,
		})
		stats.FilesChanged++
		stats.Additions += added
		stats.Deletions += removed
	}

	return files, stats
}

// ParseUnifiedDiff parses the output of `git diff HEAD` (unified format)
// into a slice of DiffFile, each with parsed hunks and lines.
func ParseUnifiedDiff(output string) []DiffFile {
	if strings.TrimSpace(output) == "" {
		return nil
	}

	var files []DiffFile
	sections := splitDiffSections(output)

	for _, section := range sections {
		file := parseDiffSection(section)
		if file.Path != "" {
			files = append(files, file)
		}
	}

	return files
}

// splitDiffSections splits unified diff output by "diff --git" headers.
func splitDiffSections(output string) []string {
	const marker = "diff --git "
	var sections []string
	var current strings.Builder
	inSection := false

	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, marker) {
			if inSection {
				sections = append(sections, current.String())
				current.Reset()
			}
			inSection = true
		}
		if inSection {
			current.WriteString(line)
			current.WriteByte('\n')
		}
	}
	if inSection && current.Len() > 0 {
		sections = append(sections, current.String())
	}

	return sections
}

// parseDiffSection parses a single file diff section into a DiffFile.
func parseDiffSection(section string) DiffFile {
	lines := strings.Split(section, "\n")
	if len(lines) == 0 {
		return DiffFile{}
	}

	path := extractFilePath(lines[0])
	var hunks []DiffHunk
	var currentHunk *DiffHunk

	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "@@") {
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
			}
			h := parseHunkHeader(line)
			currentHunk = &h
			continue
		}

		if currentHunk == nil {
			// Skip metadata lines (index, ---, +++, mode, etc.)
			continue
		}

		// Diff lines are always prefixed: ' ' (context), '+' (added), '-' (removed).
		// Bare empty lines are not part of the diff content — they come from
		// trailing newlines in the split output.
		if len(line) == 0 {
			continue
		}

		switch line[0] {
		case '+':
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				Type:    DiffAdded,
				Content: line[1:],
			})
		case '-':
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				Type:    DiffRemoved,
				Content: line[1:],
			})
		case ' ':
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				Type:    DiffContext,
				Content: line[1:],
			})
		}
	}

	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	return DiffFile{Path: path, Hunks: hunks}
}

// extractFilePath extracts the file path from a "diff --git a/path b/path" line.
func extractFilePath(line string) string {
	// Format: "diff --git a/path b/path"
	const prefix = "diff --git "
	line = strings.TrimPrefix(line, prefix)
	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	// Use the b/ side (new file path).
	return strings.TrimPrefix(parts[1], "b/")
}

// parseHunkHeader parses "@@ -old,count +new,count @@" into a DiffHunk.
func parseHunkHeader(line string) DiffHunk {
	// Format: "@@ -1,3 +1,4 @@" or "@@ -1,3 +1,4 @@ optional context"
	var oldStart, oldCount, newStart, newCount int

	// Try the full format first, then fall back to count=1 variants.
	n, _ := fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@", &oldStart, &oldCount, &newStart, &newCount)
	if n < 4 {
		// Handle single-line hunks: "@@ -1 +1,2 @@" means oldCount=1.
		fmt.Sscanf(line, "@@ -%d +%d,%d @@", &oldStart, &newStart, &newCount)
		oldCount = 1
	}

	return DiffHunk{
		OldStart: oldStart,
		OldCount: oldCount,
		NewStart: newStart,
		NewCount: newCount,
	}
}
