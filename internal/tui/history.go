package tui

// History is a pure data structure for command history navigation.
// It stores past inputs and supports Up/Down traversal.
type History struct {
	entries []string
	pos     int // current navigation position; len(entries) means "new input"
}

// NewHistory creates an empty History.
func NewHistory() *History {
	return &History{}
}

// Push adds an entry to the history and resets the navigation position.
// Empty strings and consecutive duplicates are ignored.
func (h *History) Push(s string) {
	if s == "" {
		return
	}
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == s {
		h.pos = len(h.entries)
		return
	}
	h.entries = append(h.entries, s)
	h.pos = len(h.entries)
}

// Previous moves up in history (older). Returns the entry and true if moved,
// or the current entry and false if already at the oldest.
func (h *History) Previous() (string, bool) {
	if len(h.entries) == 0 {
		return "", false
	}
	if h.pos > 0 {
		h.pos--
		return h.entries[h.pos], true
	}
	return h.entries[0], false
}

// Next moves down in history (newer). Returns the entry and true if moved.
// When moving past the newest entry, returns empty string (back to new input).
func (h *History) Next() (string, bool) {
	if h.pos >= len(h.entries) {
		return "", false
	}
	h.pos++
	if h.pos >= len(h.entries) {
		return "", true
	}
	return h.entries[h.pos], true
}
