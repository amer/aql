package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func TestHistory_EmptyPrevious(t *testing.T) {
	h := tui.NewHistory()
	val, ok := h.Previous()
	assert.False(t, ok)
	assert.Equal(t, "", val)
}

func TestHistory_EmptyNext(t *testing.T) {
	h := tui.NewHistory()
	val, ok := h.Next()
	assert.False(t, ok)
	assert.Equal(t, "", val)
}

func TestHistory_PushAndPrevious(t *testing.T) {
	h := tui.NewHistory()
	h.Push("first")
	h.Push("second")
	h.Push("third")

	val, ok := h.Previous()
	assert.True(t, ok)
	assert.Equal(t, "third", val)

	val, ok = h.Previous()
	assert.True(t, ok)
	assert.Equal(t, "second", val)

	val, ok = h.Previous()
	assert.True(t, ok)
	assert.Equal(t, "first", val)

	// Past the beginning — stays at first
	val, ok = h.Previous()
	assert.False(t, ok)
	assert.Equal(t, "first", val)
}

func TestHistory_PreviousThenNext(t *testing.T) {
	h := tui.NewHistory()
	h.Push("alpha")
	h.Push("beta")

	h.Previous() // beta
	h.Previous() // alpha

	val, ok := h.Next()
	assert.True(t, ok)
	assert.Equal(t, "beta", val)

	// Past the end — returns empty (back to new input)
	val, ok = h.Next()
	assert.True(t, ok)
	assert.Equal(t, "", val)

	// Already at bottom
	val, ok = h.Next()
	assert.False(t, ok)
	assert.Equal(t, "", val)
}

func TestHistory_ResetOnPush(t *testing.T) {
	h := tui.NewHistory()
	h.Push("a")
	h.Push("b")

	h.Previous() // b
	h.Previous() // a

	// Pushing resets the navigation position
	h.Push("c")

	val, ok := h.Previous()
	assert.True(t, ok)
	assert.Equal(t, "c", val)
}

func TestHistory_SkipDuplicates(t *testing.T) {
	h := tui.NewHistory()
	h.Push("same")
	h.Push("same")

	val, ok := h.Previous()
	assert.True(t, ok)
	assert.Equal(t, "same", val)

	// Only one entry — can't go further back
	_, ok = h.Previous()
	assert.False(t, ok)
}

func TestHistory_SkipEmpty(t *testing.T) {
	h := tui.NewHistory()
	h.Push("")
	h.Push("real")

	val, ok := h.Previous()
	assert.True(t, ok)
	assert.Equal(t, "real", val)

	// Empty string was not stored
	_, ok = h.Previous()
	assert.False(t, ok)
}
