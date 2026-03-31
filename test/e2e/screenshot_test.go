package e2e_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amer/aql/test/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScreenshot_Contains(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		substr string
		want   bool
	}{
		{"found", "hello world", "world", true},
		{"not found", "hello world", "foo", false},
		{"empty substr", "hello", "", true},
		{"empty text", "", "foo", false},
		{"multiline", "line1\nline2\nline3", "line2", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := e2e.NewScreenshot(tt.text, time.Now())
			assert.Equal(t, tt.want, s.Contains(tt.substr))
		})
	}
}

func TestScreenshot_Line(t *testing.T) {
	s := e2e.NewScreenshot("line0\nline1\nline2", time.Now())

	assert.Equal(t, "line0", s.Line(0))
	assert.Equal(t, "line1", s.Line(1))
	assert.Equal(t, "line2", s.Line(2))
	assert.Equal(t, "", s.Line(3), "out of bounds returns empty")
	assert.Equal(t, "", s.Line(-1), "negative returns empty")
}

func TestScreenshot_Lines(t *testing.T) {
	s := e2e.NewScreenshot("a\nb\nc", time.Now())
	assert.Equal(t, []string{"a", "b", "c"}, s.Lines)
}

func TestScreenshot_Save(t *testing.T) {
	s := e2e.NewScreenshot("hello\nworld", time.Now())
	path := filepath.Join(t.TempDir(), "shot.txt")

	err := s.Save(path)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello\nworld\n", string(data))
}
