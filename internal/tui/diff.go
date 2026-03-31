package tui

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - diffState struct, diffMode enum, diff overlay rendering
//     (RenderDiffFileList, RenderDiffDetail), diff key handling
//     (handleDiffKey), testing accessors (DiffVisible, DiffFiles).
//
// MUST NOT GO HERE:
//   - Git execution or diff parsing (internal/diff package), agent
//     imports, DiffResultMsg type definition (types.go).
//
// Q: How do I add a new view mode to the diff overlay?
// A: Add a constant to diffMode, handle it in handleDiffKey and
//    renderDiffOverlay.
// ──────────────────────────────────────────────────────────────────

import (
	"fmt"
	"strings"

	"github.com/amer/aql/internal/diff"
)

// diffMode identifies which view the diff overlay is showing.
type diffMode int

const (
	diffModeList   diffMode = iota // file list with stats
	diffModeDetail                 // single file hunk view
)

// diffState holds the TUI-side diff overlay state.
type diffState struct {
	visible   bool
	mode      diffMode
	files     []diff.DiffFile
	stats     diff.DiffStats
	selected  int // cursor in file list
	scrollTop int // scroll offset in detail view
	loading   bool
}

// --- Testing accessors ---

// DiffVisible returns whether the diff overlay is visible (for testing).
func (m Model) DiffVisible() bool {
	return m.diffPanel.visible
}

// DiffFiles returns the current diff file list (for testing).
func (m Model) DiffFiles() []diff.DiffFile {
	return m.diffPanel.files
}

// --- Rendering ---

// RenderDiffFileList renders the file list view of the diff overlay.
func RenderDiffFileList(files []diff.DiffFile, stats diff.DiffStats, selected, width int) string {
	if len(files) == 0 {
		return DimStyle.Render("  No changed files")
	}

	var b strings.Builder

	// Header with stats
	header := fmt.Sprintf("  %d %s changed", stats.FilesChanged, filePlural(stats.FilesChanged))
	if stats.Additions > 0 {
		header += DiffAddedStyle.Render(fmt.Sprintf(" +%d", stats.Additions))
	}
	if stats.Deletions > 0 {
		header += DiffRemovedStyle.Render(fmt.Sprintf(" -%d", stats.Deletions))
	}
	b.WriteString(header)
	b.WriteString("\n\n")

	// File list
	for i, f := range files {
		prefix := "  "
		if i == selected {
			prefix = DiffSelectedStyle.Render("▸ ")
		}
		b.WriteString(prefix)
		b.WriteString(fileLine(f, i == selected, width-4))
		b.WriteString("\n")
	}

	// Footer hints
	b.WriteString("\n")
	b.WriteString(DimStyle.Render("  ↑/↓ select  enter view  esc close"))

	return b.String()
}

// RenderDiffDetail renders the detail hunk view for a single file.
func RenderDiffDetail(file diff.DiffFile, scrollTop, height, width int) string {
	var b strings.Builder

	// File header
	b.WriteString("  ")
	b.WriteString(BoldStyle.Render(file.Path))
	b.WriteString("\n")
	b.WriteString(DimStyle.Render(strings.Repeat("─", min(width, 80))))
	b.WriteString("\n")

	if file.IsBinary {
		b.WriteString(DimStyle.Render("  Binary file — cannot display diff"))
		return b.String()
	}

	if len(file.Hunks) == 0 {
		b.WriteString(DimStyle.Render("  No diff content"))
		return b.String()
	}

	// Render all hunk lines into a buffer, then apply scroll window.
	var lines []string
	for hi, hunk := range file.Hunks {
		if hi > 0 {
			lines = append(lines, DimStyle.Render("  ···"))
		}
		hunkHeader := fmt.Sprintf("@@ -%d,%d +%d,%d @@",
			hunk.OldStart, hunk.OldCount, hunk.NewStart, hunk.NewCount)
		lines = append(lines, DiffHunkHeaderStyle.Render("  "+hunkHeader))

		for _, dl := range hunk.Lines {
			lines = append(lines, renderDiffLine(dl))
		}
	}

	// Apply scroll window.
	if scrollTop > len(lines) {
		scrollTop = len(lines)
	}
	end := scrollTop + height
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[scrollTop:end]

	for _, line := range visible {
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(DimStyle.Render("  esc back"))

	return b.String()
}

// fileLine renders a single file entry with path and stats.
func fileLine(f diff.DiffFile, selected bool, maxWidth int) string {
	path := f.Path
	if len(path) > maxWidth-15 {
		path = "…" + path[len(path)-(maxWidth-16):]
	}

	var stats string
	if f.IsBinary {
		stats = DimStyle.Render("binary")
	} else {
		var parts []string
		if f.LinesAdded > 0 {
			parts = append(parts, DiffAddedStyle.Render(fmt.Sprintf("+%d", f.LinesAdded)))
		}
		if f.LinesRemoved > 0 {
			parts = append(parts, DiffRemovedStyle.Render(fmt.Sprintf("-%d", f.LinesRemoved)))
		}
		stats = strings.Join(parts, " ")
	}

	if selected {
		return BoldStyle.Render(path) + "  " + stats
	}
	return path + "  " + stats
}

// renderDiffLine renders a single diff line with the appropriate style.
func renderDiffLine(dl diff.DiffLine) string {
	switch dl.Type {
	case diff.DiffAdded:
		return DiffAddedStyle.Render("  + " + dl.Content)
	case diff.DiffRemoved:
		return DiffRemovedStyle.Render("  - " + dl.Content)
	default:
		return "    " + dl.Content
	}
}

func filePlural(n int) string {
	if n == 1 {
		return "file"
	}
	return "files"
}
