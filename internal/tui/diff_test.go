package tui_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/amer/aql/internal/diff"
	"github.com/amer/aql/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffOverlay_navigation(t *testing.T) {
	onSubmit := func(input string) tea.Cmd { return nil }
	m := tui.NewModel("test", []string{"agent"}, onSubmit)

	// Simulate DiffResultMsg
	files := []diff.DiffFile{
		{Path: "a.go", LinesAdded: 5, LinesRemoved: 2, Hunks: []diff.DiffHunk{
			{OldStart: 1, OldCount: 3, NewStart: 1, NewCount: 4, Lines: []diff.DiffLine{
				{Type: diff.DiffContext, Content: "line1"},
				{Type: diff.DiffAdded, Content: "line2"},
			}},
		}},
		{Path: "b.go", LinesAdded: 3, LinesRemoved: 0},
	}
	msg := tui.DiffResultMsg{
		Files: files,
		Stats: diff.DiffStats{FilesChanged: 2, Additions: 8, Deletions: 2},
	}

	result, _ := m.Update(msg)
	m = result.(tui.Model)
	assert.True(t, m.DiffVisible(), "diff should be visible after DiffResultMsg")
	assert.Len(t, m.DiffFiles(), 2)

	// Navigate down
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(tui.Model)

	// Enter detail view
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(tui.Model)

	// Back to list (esc)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = result.(tui.Model)
	assert.True(t, m.DiffVisible(), "should still be visible after esc from detail")

	// Close overlay (esc from list)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = result.(tui.Model)
	assert.False(t, m.DiffVisible(), "should be closed after esc from list")
}

func TestDiffOverlay_error(t *testing.T) {
	onSubmit := func(input string) tea.Cmd { return nil }
	m := tui.NewModel("test", []string{"agent"}, onSubmit)

	msg := tui.DiffResultMsg{
		Err: errors.New("git not found"),
	}
	result, _ := m.Update(msg)
	m = result.(tui.Model)
	assert.False(t, m.DiffVisible(), "diff should not be visible on error")
}

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

func TestRenderDiffDetail_lineNumbers(t *testing.T) {
	// The diff detail view should show line numbers and sigils like Claude Code:
	//   1  package main
	//   2 -// old
	//   2 +// new
	//   3  func main() {}
	file := diff.DiffFile{
		Path: "main.go",
		Hunks: []diff.DiffHunk{
			{
				OldStart: 1, OldCount: 3,
				NewStart: 1, NewCount: 3,
				Lines: []diff.DiffLine{
					{Type: diff.DiffContext, Content: "package main"},
					{Type: diff.DiffRemoved, Content: "// old"},
					{Type: diff.DiffAdded, Content: "// new"},
					{Type: diff.DiffContext, Content: "func main() {}"},
				},
			},
		},
	}

	result := tui.RenderDiffDetail(file, 0, 40, 80)

	// Line numbers should be present and right-aligned
	assert.Contains(t, result, "1 ", "should have line number 1")
	// Removed lines get a - sigil
	assert.Contains(t, result, "-// old", "removed lines should have - sigil")
	// Added lines get a + sigil
	assert.Contains(t, result, "+// new", "added lines should have + sigil")
	// Context lines have space sigil (no +/-)
	assert.Contains(t, result, " package main", "context lines should have space sigil")
}

func TestRenderDiffDetail_lineNumberTracking(t *testing.T) {
	// Removed lines show old line numbers, added lines show new line numbers.
	// After a removal block, the new line numbers adjust.
	//   10  context before
	//   11 -removed line
	//   11 +added line 1
	//   12 +added line 2
	//   13  context after
	file := diff.DiffFile{
		Path: "test.go",
		Hunks: []diff.DiffHunk{
			{
				OldStart: 10, OldCount: 3,
				NewStart: 10, NewCount: 4,
				Lines: []diff.DiffLine{
					{Type: diff.DiffContext, Content: "context before"},
					{Type: diff.DiffRemoved, Content: "removed line"},
					{Type: diff.DiffAdded, Content: "added line 1"},
					{Type: diff.DiffAdded, Content: "added line 2"},
					{Type: diff.DiffContext, Content: "context after"},
				},
			},
		},
	}

	result := tui.RenderDiffDetail(file, 0, 40, 80)

	// Should contain line numbers 10-13
	assert.Contains(t, result, "10", "should have line number 10")
	assert.Contains(t, result, "11", "should have line number 11")
	assert.Contains(t, result, "12", "should have line number 12")
	assert.Contains(t, result, "13", "should have line number 13")
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
