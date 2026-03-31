package tui_test

import (
	"strings"
	"testing"

	"github.com/amer/aql/internal/diff"
	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderDiffFileList(t *testing.T) {
	tests := []struct {
		name     string
		files    []diff.DiffFile
		stats    diff.DiffStats
		selected int
		width    int
		contains []string
		absent   []string
	}{
		{
			name:     "empty file list",
			files:    nil,
			stats:    diff.DiffStats{},
			selected: 0,
			width:    80,
			contains: []string{"No changed files"},
		},
		{
			name: "single file selected",
			files: []diff.DiffFile{
				{Path: "main.go", LinesAdded: 10, LinesRemoved: 3},
			},
			stats:    diff.DiffStats{FilesChanged: 1, Additions: 10, Deletions: 3},
			selected: 0,
			width:    80,
			contains: []string{"main.go", "+10", "-3", "1 file"},
		},
		{
			name: "multiple files with selection",
			files: []diff.DiffFile{
				{Path: "a.go", LinesAdded: 5, LinesRemoved: 2},
				{Path: "b.go", LinesAdded: 0, LinesRemoved: 8},
				{Path: "c.go", LinesAdded: 3, LinesRemoved: 0},
			},
			stats:    diff.DiffStats{FilesChanged: 3, Additions: 8, Deletions: 10},
			selected: 1,
			width:    80,
			contains: []string{"a.go", "b.go", "c.go", "3 files"},
		},
		{
			name: "binary file",
			files: []diff.DiffFile{
				{Path: "image.png", IsBinary: true},
			},
			stats:    diff.DiffStats{FilesChanged: 1},
			selected: 0,
			width:    80,
			contains: []string{"image.png", "binary"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tui.RenderDiffFileList(tt.files, tt.stats, tt.selected, tt.width)
			for _, s := range tt.contains {
				assert.Contains(t, result, s, "expected %q in output", s)
			}
			for _, s := range tt.absent {
				assert.NotContains(t, result, s, "unexpected %q in output", s)
			}
		})
	}
}

func TestRenderDiffDetail(t *testing.T) {
	tests := []struct {
		name     string
		file     diff.DiffFile
		width    int
		contains []string
	}{
		{
			name:     "file with no hunks",
			file:     diff.DiffFile{Path: "empty.go"},
			width:    80,
			contains: []string{"empty.go", "No diff content"},
		},
		{
			name: "file with hunks",
			file: diff.DiffFile{
				Path: "main.go",
				Hunks: []diff.DiffHunk{
					{
						OldStart: 1, OldCount: 3,
						NewStart: 1, NewCount: 4,
						Lines: []diff.DiffLine{
							{Type: diff.DiffContext, Content: "package main"},
							{Type: diff.DiffRemoved, Content: "// old"},
							{Type: diff.DiffAdded, Content: "// new"},
							{Type: diff.DiffContext, Content: "func main() {}"},
						},
					},
				},
			},
			width:    80,
			contains: []string{"main.go", "package main", "// old", "// new", "func main() {}"},
		},
		{
			name:     "binary file",
			file:     diff.DiffFile{Path: "image.png", IsBinary: true},
			width:    80,
			contains: []string{"image.png", "Binary file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tui.RenderDiffDetail(tt.file, 0, 40, tt.width)
			for _, s := range tt.contains {
				assert.Contains(t, result, s, "expected %q in output", s)
			}
		})
	}
}

func TestRenderDiffDetail_scrolling(t *testing.T) {
	// Create a file with many lines
	var lines []diff.DiffLine
	for i := 0; i < 50; i++ {
		lines = append(lines, diff.DiffLine{
			Type:    diff.DiffContext,
			Content: strings.Repeat("x", 10),
		})
	}
	file := diff.DiffFile{
		Path: "big.go",
		Hunks: []diff.DiffHunk{
			{OldStart: 1, OldCount: 50, NewStart: 1, NewCount: 50, Lines: lines},
		},
	}

	full := tui.RenderDiffDetail(file, 0, 100, 80)
	scrolled := tui.RenderDiffDetail(file, 10, 20, 80)

	// Scrolled view should have fewer lines
	fullLines := strings.Split(full, "\n")
	scrolledLines := strings.Split(scrolled, "\n")
	require.Less(t, len(scrolledLines), len(fullLines))
}
