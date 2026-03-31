package tui

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - InputBuffer — pure data structure for cursor-aware text
//     editing with readline operations (Insert, DeleteBackward,
//     MoveLeft/Right, KillToEnd/Start, Clear, Set),
//     RenderWithCursor.
//
// MUST NOT GO HERE:
//   - Keyboard handling (handlers.go), TUI state, anything beyond
//     pure buffer operations. This is functional core.
//
// Q: Should I add a new editing operation?
// A: Add a method to InputBuffer here. Call it from handlers.go.
// ──────────────────────────────────────────────────────────────────

import "strings"

// InputBuffer is a pure data structure for cursor-aware text editing.
// It tracks the buffer content and cursor position, supporting
// readline-style operations (insert, delete, move, kill).
type InputBuffer struct {
	runes  []rune
	cursor int
}

// NewInputBuffer creates an empty InputBuffer with cursor at position 0.
func NewInputBuffer() *InputBuffer {
	return &InputBuffer{}
}

// Insert inserts a rune at the current cursor position and advances the cursor.
func (b *InputBuffer) Insert(r rune) {
	b.runes = append(b.runes, 0)
	copy(b.runes[b.cursor+1:], b.runes[b.cursor:])
	b.runes[b.cursor] = r
	b.cursor++
}

// InsertString inserts all runes of s at the current cursor position.
func (b *InputBuffer) InsertString(s string) {
	rs := []rune(s)
	if len(rs) == 0 {
		return
	}
	newRunes := make([]rune, len(b.runes)+len(rs))
	copy(newRunes, b.runes[:b.cursor])
	copy(newRunes[b.cursor:], rs)
	copy(newRunes[b.cursor+len(rs):], b.runes[b.cursor:])
	b.runes = newRunes
	b.cursor += len(rs)
}

// DeleteBackward deletes the rune before the cursor (backspace). No-op at position 0.
func (b *InputBuffer) DeleteBackward() {
	if b.cursor == 0 {
		return
	}
	b.runes = append(b.runes[:b.cursor-1], b.runes[b.cursor:]...)
	b.cursor--
}

// MoveLeft moves the cursor one position left. No-op at position 0.
func (b *InputBuffer) MoveLeft() {
	if b.cursor > 0 {
		b.cursor--
	}
}

// MoveRight moves the cursor one position right. No-op at end.
func (b *InputBuffer) MoveRight() {
	if b.cursor < len(b.runes) {
		b.cursor++
	}
}

// MoveToStart moves the cursor to position 0.
func (b *InputBuffer) MoveToStart() {
	b.cursor = 0
}

// MoveToEnd moves the cursor to the end of the buffer.
func (b *InputBuffer) MoveToEnd() {
	b.cursor = len(b.runes)
}

// KillToEnd deletes all content from the cursor to the end of the buffer.
func (b *InputBuffer) KillToEnd() {
	b.runes = b.runes[:b.cursor]
}

// KillToStart deletes all content from the start to the cursor position.
func (b *InputBuffer) KillToStart() {
	b.runes = b.runes[b.cursor:]
	b.cursor = 0
}

// Clear resets the buffer to empty.
func (b *InputBuffer) Clear() {
	b.runes = nil
	b.cursor = 0
}

// Set replaces the buffer content and moves the cursor to the end.
func (b *InputBuffer) Set(s string) {
	b.runes = []rune(s)
	b.cursor = len(b.runes)
}

// String returns the buffer content as a string.
func (b *InputBuffer) String() string {
	return string(b.runes)
}

// Cursor returns the current cursor position.
func (b *InputBuffer) Cursor() int {
	return b.cursor
}

// RenderWithCursor returns the buffer content with a block cursor (█)
// at the current cursor position. If the cursor is at the end, the block
// is appended. If mid-text, the character under the cursor is replaced
// with the block character.
func (b *InputBuffer) RenderWithCursor() string {
	if b.cursor >= len(b.runes) {
		return string(b.runes) + "█"
	}
	var sb strings.Builder
	for i, r := range b.runes {
		if i == b.cursor {
			sb.WriteRune('█')
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
