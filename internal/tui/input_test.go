package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
)

func TestInputBuffer_InsertAtEnd(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.Insert('h')
	buf.Insert('i')
	assert.Equal(t, "hi", buf.String())
	assert.Equal(t, 2, buf.Cursor())
}

func TestInputBuffer_InsertAtMiddle(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.Insert('a')
	buf.Insert('c')
	buf.MoveLeft()
	buf.Insert('b')
	assert.Equal(t, "abc", buf.String())
	assert.Equal(t, 2, buf.Cursor())
}

func TestInputBuffer_DeleteBackward(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.Insert('a')
	buf.Insert('b')
	buf.Insert('c')
	buf.DeleteBackward()
	assert.Equal(t, "ab", buf.String())
	assert.Equal(t, 2, buf.Cursor())
}

func TestInputBuffer_DeleteBackwardAtStart(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.Insert('a')
	buf.MoveToStart()
	buf.DeleteBackward() // no-op at position 0
	assert.Equal(t, "a", buf.String())
	assert.Equal(t, 0, buf.Cursor())
}

func TestInputBuffer_DeleteBackwardMiddle(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.Insert('a')
	buf.Insert('b')
	buf.Insert('c')
	buf.MoveLeft()
	buf.DeleteBackward()
	assert.Equal(t, "ac", buf.String())
	assert.Equal(t, 1, buf.Cursor())
}

func TestInputBuffer_MoveLeft(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.Insert('a')
	buf.Insert('b')
	buf.MoveLeft()
	assert.Equal(t, 1, buf.Cursor())
	buf.MoveLeft()
	assert.Equal(t, 0, buf.Cursor())
	buf.MoveLeft() // no-op at 0
	assert.Equal(t, 0, buf.Cursor())
}

func TestInputBuffer_MoveRight(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.Insert('a')
	buf.Insert('b')
	buf.MoveToStart()
	buf.MoveRight()
	assert.Equal(t, 1, buf.Cursor())
	buf.MoveRight()
	assert.Equal(t, 2, buf.Cursor())
	buf.MoveRight() // no-op at end
	assert.Equal(t, 2, buf.Cursor())
}

func TestInputBuffer_MoveToStart(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.Insert('a')
	buf.Insert('b')
	buf.Insert('c')
	buf.MoveToStart()
	assert.Equal(t, 0, buf.Cursor())
}

func TestInputBuffer_MoveToEnd(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.Insert('a')
	buf.Insert('b')
	buf.MoveToStart()
	buf.MoveToEnd()
	assert.Equal(t, 2, buf.Cursor())
}

func TestInputBuffer_KillToEnd(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.Insert('a')
	buf.Insert('b')
	buf.Insert('c')
	buf.MoveToStart()
	buf.MoveRight()
	buf.KillToEnd()
	assert.Equal(t, "a", buf.String())
	assert.Equal(t, 1, buf.Cursor())
}

func TestInputBuffer_KillToStart(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.Insert('a')
	buf.Insert('b')
	buf.Insert('c')
	buf.MoveLeft()
	buf.KillToStart()
	assert.Equal(t, "c", buf.String())
	assert.Equal(t, 0, buf.Cursor())
}

func TestInputBuffer_Clear(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.Insert('a')
	buf.Insert('b')
	buf.Clear()
	assert.Equal(t, "", buf.String())
	assert.Equal(t, 0, buf.Cursor())
}

func TestInputBuffer_Set(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.Set("hello")
	assert.Equal(t, "hello", buf.String())
	assert.Equal(t, 5, buf.Cursor())
}

func TestInputBuffer_InsertNewline(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.Insert('a')
	buf.Insert('\n')
	buf.Insert('b')
	assert.Equal(t, "a\nb", buf.String())
}

func TestInputBuffer_RenderWithCursor(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.Set("hello")
	// Cursor at end: "hello█"
	assert.Equal(t, "hello█", buf.RenderWithCursor())

	buf.MoveToStart()
	// Cursor at start: "█hello" — but we render the char under cursor differently
	// The cursor replaces the char at position with a block
	rendered := buf.RenderWithCursor()
	assert.Contains(t, rendered, "█")
}

func TestInputBuffer_InsertStringAtEnd(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.InsertString("hello world")
	assert.Equal(t, "hello world", buf.String())
	assert.Equal(t, 11, buf.Cursor())
}

func TestInputBuffer_InsertStringAtMiddle(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.InsertString("hd")
	buf.MoveLeft()
	buf.InsertString("ello worl")
	assert.Equal(t, "hello world", buf.String())
	assert.Equal(t, 10, buf.Cursor())
}

func TestInputBuffer_InsertStringEmpty(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.InsertString("existing")
	buf.InsertString("")
	assert.Equal(t, "existing", buf.String())
	assert.Equal(t, 8, buf.Cursor())
}

func TestInputBuffer_InsertStringMultiline(t *testing.T) {
	buf := tui.NewInputBuffer()
	buf.InsertString("line1\nline2\nline3")
	assert.Equal(t, "line1\nline2\nline3", buf.String())
	assert.Equal(t, 17, buf.Cursor())
}

func TestInputBuffer_Empty(t *testing.T) {
	buf := tui.NewInputBuffer()
	assert.Equal(t, "", buf.String())
	assert.Equal(t, 0, buf.Cursor())
	assert.Equal(t, "█", buf.RenderWithCursor())
}
