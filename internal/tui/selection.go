package tui

import (
	"strings"
	"unicode/utf8"
)

// Selection tracks a text selection region by screen coordinates.
// Coordinates are 0-based: X is the column, Y is the row.
type Selection struct {
	startX, startY int
	endX, endY     int
	active         bool
}

// Start begins a new selection at the given screen position.
func (s *Selection) Start(x, y int) {
	s.startX = x
	s.startY = y
	s.endX = x
	s.endY = y
	s.active = true
}

// Update extends the selection to the given screen position.
func (s *Selection) Update(x, y int) {
	s.endX = x
	s.endY = y
}

// Clear cancels the selection.
func (s *Selection) Clear() {
	s.active = false
}

// Active returns true if a selection is in progress.
func (s Selection) Active() bool {
	return s.active
}

// Normalized returns start and end in top-to-bottom, left-to-right order.
func (s Selection) Normalized() (sx, sy, ex, ey int) {
	sx, sy, ex, ey = s.startX, s.startY, s.endX, s.endY
	if sy > ey || (sy == ey && sx > ex) {
		sx, sy, ex, ey = ex, ey, sx, sy
	}
	return
}

// Extract returns the selected text from the given screen lines.
func (s Selection) Extract(lines []string) string {
	if !s.active || len(lines) == 0 {
		return ""
	}

	sx, sy, ex, ey := s.Normalized()

	// Clamp to line bounds
	if sy >= len(lines) {
		return ""
	}
	if ey >= len(lines) {
		ey = len(lines) - 1
		ex = len(lines[ey])
	}

	if sy == ey {
		line := lines[sy]
		sxB := runeOffset(line, sx)
		exB := runeOffset(line, ex)
		if sxB >= exB {
			return ""
		}
		return line[sxB:exB]
	}

	var b strings.Builder

	// First line: from startX to end
	first := lines[sy]
	sxB := runeOffset(first, sx)
	b.WriteString(first[sxB:])

	// Middle lines: full lines
	for i := sy + 1; i < ey; i++ {
		b.WriteString("\n")
		b.WriteString(lines[i])
	}

	// Last line: from start to endX
	last := lines[ey]
	exB := runeOffset(last, ex)
	b.WriteString("\n")
	b.WriteString(last[:exB])

	return b.String()
}

// runeOffset returns the byte offset of the n-th rune in s.
// If n exceeds the rune count, returns len(s).
func runeOffset(s string, n int) int {
	off := 0
	for i := 0; i < n; i++ {
		if off >= len(s) {
			return len(s)
		}
		_, size := utf8.DecodeRuneInString(s[off:])
		off += size
	}
	return off
}

// Contains returns true if the given screen position is inside the selection.
func (s Selection) Contains(x, y int) bool {
	if !s.active {
		return false
	}

	sx, sy, ex, ey := s.Normalized()

	if y < sy || y > ey {
		return false
	}
	if sy == ey {
		return x >= sx && x < ex
	}
	if y == sy {
		return x >= sx
	}
	if y == ey {
		return x < ex
	}
	return true // middle line — fully selected
}
