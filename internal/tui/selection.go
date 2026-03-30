package tui

import "strings"

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
		if sx > len(line) {
			sx = len(line)
		}
		if ex > len(line) {
			ex = len(line)
		}
		if sx >= ex {
			return ""
		}
		return line[sx:ex]
	}

	var b strings.Builder

	// First line: from startX to end
	first := lines[sy]
	if sx > len(first) {
		sx = len(first)
	}
	b.WriteString(first[sx:])

	// Middle lines: full lines
	for i := sy + 1; i < ey; i++ {
		b.WriteString("\n")
		b.WriteString(lines[i])
	}

	// Last line: from start to endX
	last := lines[ey]
	if ex > len(last) {
		ex = len(last)
	}
	b.WriteString("\n")
	b.WriteString(last[:ex])

	return b.String()
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
