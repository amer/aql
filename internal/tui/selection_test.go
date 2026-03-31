package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func TestSelection_Empty(t *testing.T) {
	s := tui.Selection{}
	assert.False(t, s.Active(), "empty selection should not be active")
	assert.Equal(t, "", s.Extract(nil))
}

func TestSelection_SingleLine(t *testing.T) {
	s := tui.Selection{}
	s.Start(6, 2)
	s.Update(11, 2)

	assert.True(t, s.Active())

	lines := []string{
		"first line",
		"second line",
		"hello world here",
		"fourth line",
	}
	assert.Equal(t, "world", s.Extract(lines))
}

func TestSelection_MultiLine(t *testing.T) {
	s := tui.Selection{}
	s.Start(6, 0)
	s.Update(5, 2)

	lines := []string{
		"first line",
		"second line",
		"third line",
	}
	assert.Equal(t, "line\nsecond line\nthird", s.Extract(lines))
}

func TestSelection_ReverseDirection(t *testing.T) {
	// Select from bottom-right to top-left
	s := tui.Selection{}
	s.Start(10, 2)
	s.Update(6, 0)

	lines := []string{
		"first line",
		"second line",
		"third line",
	}
	assert.Equal(t, "line\nsecond line\nthird line", s.Extract(lines))
}

func TestSelection_Clear(t *testing.T) {
	s := tui.Selection{}
	s.Start(5, 2)
	s.Update(10, 2)
	assert.True(t, s.Active())

	s.Clear()
	assert.False(t, s.Active())
	assert.Equal(t, "", s.Extract(nil))
}

func TestSelection_OutOfBounds(t *testing.T) {
	s := tui.Selection{}
	s.Start(0, 0)
	s.Update(100, 10) // way past content

	lines := []string{
		"short",
		"line",
	}
	got := s.Extract(lines)
	assert.Equal(t, "short\nline", got)
}

func TestSelection_SingleChar(t *testing.T) {
	s := tui.Selection{}
	s.Start(3, 0)
	s.Update(4, 0)

	lines := []string{"hello"}
	assert.Equal(t, "l", s.Extract(lines))
}

func TestSelection_WholeLineByX(t *testing.T) {
	s := tui.Selection{}
	s.Start(0, 1)
	s.Update(11, 1)

	lines := []string{
		"first",
		"second line",
		"third",
	}
	assert.Equal(t, "second line", s.Extract(lines))
}

func TestSelection_ContainsPoint(t *testing.T) {
	s := tui.Selection{}
	s.Start(2, 1)
	s.Update(8, 1)

	assert.True(t, s.Contains(3, 1), "point inside selection")
	assert.True(t, s.Contains(2, 1), "start point")
	assert.False(t, s.Contains(8, 1), "end point is exclusive")
	assert.False(t, s.Contains(3, 0), "wrong line")
	assert.False(t, s.Contains(3, 2), "wrong line")
}

func TestSelection_MultiByte(t *testing.T) {
	// Screen columns are rune-based, not byte-based.
	// "⏺" is a 3-byte UTF-8 character but occupies 1 screen column.
	s := tui.Selection{}
	s.Start(1, 0) // after the ⏺ character (column 1)
	s.Update(7, 0)

	lines := []string{
		"⏺      Here's the project root",
	}
	// Should get 6 chars: "      " (spaces), not garbled UTF-8 bytes
	assert.Equal(t, "      ", s.Extract(lines))
}

func TestSelection_MultiByteMiddle(t *testing.T) {
	// Selecting text that spans across multi-byte characters
	s := tui.Selection{}
	s.Start(0, 0)
	s.Update(3, 0)

	lines := []string{
		"⏺⎿hello",
	}
	// Should get "⏺⎿h" (3 runes), not garbled bytes
	assert.Equal(t, "⏺⎿h", s.Extract(lines))
}

func TestSelection_ContainsMultiLine(t *testing.T) {
	s := tui.Selection{}
	s.Start(5, 0)
	s.Update(3, 2)

	assert.True(t, s.Contains(5, 0), "start of first line")
	assert.True(t, s.Contains(0, 1), "start of middle line")
	assert.True(t, s.Contains(99, 1), "end of middle line")
	assert.True(t, s.Contains(2, 2), "before end on last line")
	assert.False(t, s.Contains(3, 2), "at end (exclusive)")
	assert.False(t, s.Contains(4, 0), "before start on first line")
}
