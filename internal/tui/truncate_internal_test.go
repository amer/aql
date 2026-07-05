package tui

import (
	"testing"

	"github.com/amer/aql/internal/diff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A narrow width leaves too little room for the ", +more" suffix. The
// truncation math must not slice past the start of the string.
func TestRenderConnectorLine_NarrowWidthDoesNotPanic(t *testing.T) {
	for width := 0; width <= 20; width++ {
		require.NotPanics(t, func() {
			renderConnectorLine("a very long summary that exceeds width", width)
		}, "width=%d", width)
	}
}

// Long paths in a narrow file list must truncate without slicing out of range.
func TestFileLine_NarrowWidthDoesNotPanic(t *testing.T) {
	f := diff.DiffFile{Path: "internal/tui/some/deeply/nested/path.go"}
	for width := 0; width <= 20; width++ {
		require.NotPanics(t, func() {
			fileLine(f, false, width)
		}, "width=%d", width)
	}
}

func TestTruncateEnd(t *testing.T) {
	assert.Equal(t, "", truncateEnd("hello", 0))
	assert.Equal(t, "hello", truncateEnd("hello", 10))
	assert.Equal(t, "hel", truncateEnd("hello", 3)) // no room for suffix
	assert.Equal(t, "ab, +more", truncateEnd("abcdefghij", 9))
	assert.Equal(t, "héllo", truncateEnd("héllo", 5)) // rune-safe, no split
}

func TestTruncateTail(t *testing.T) {
	assert.Equal(t, "", truncateTail("hello", 0))
	assert.Equal(t, "hello", truncateTail("hello", 10))
	assert.Equal(t, "…", truncateTail("hello", 1))
	assert.Equal(t, "…llo", truncateTail("hello", 4))
	assert.Equal(t, "…llo", truncateTail("héllo", 4)) // rune-safe tail
}

// Multibyte characters (accents, CJK, emoji) must reach the input buffer as a
// single rune rather than being dropped for having len(bytes) != 1.
func TestKeyToRune_Multibyte(t *testing.T) {
	got, ok := keyToRune("é")
	require.True(t, ok)
	assert.Equal(t, 'é', got)

	got, ok = keyToRune("世")
	require.True(t, ok)
	assert.Equal(t, '世', got)

	_, ok = keyToRune("ab")
	assert.False(t, ok, "multi-rune strings are not single-key input")

	_, ok = keyToRune("")
	assert.False(t, ok)
}
