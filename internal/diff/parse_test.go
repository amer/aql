package diff_test

import (
	"testing"

	"github.com/amer/aql/internal/diff"
	"github.com/stretchr/testify/assert"
)

func TestParseNumstat(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantFiles []diff.DiffFile
		wantStats diff.DiffStats
	}{
		{
			name:      "empty output",
			input:     "",
			wantFiles: nil,
			wantStats: diff.DiffStats{},
		},
		{
			name:  "single file",
			input: "10\t3\tinternal/tui/app.go\n",
			wantFiles: []diff.DiffFile{
				{Path: "internal/tui/app.go", LinesAdded: 10, LinesRemoved: 3},
			},
			wantStats: diff.DiffStats{FilesChanged: 1, Additions: 10, Deletions: 3},
		},
		{
			name:  "multiple files",
			input: "5\t2\tREADME.md\n20\t0\tinternal/diff/types.go\n0\t15\told_file.go\n",
			wantFiles: []diff.DiffFile{
				{Path: "README.md", LinesAdded: 5, LinesRemoved: 2},
				{Path: "internal/diff/types.go", LinesAdded: 20, LinesRemoved: 0},
				{Path: "old_file.go", LinesAdded: 0, LinesRemoved: 15},
			},
			wantStats: diff.DiffStats{FilesChanged: 3, Additions: 25, Deletions: 17},
		},
		{
			name:  "binary file",
			input: "-\t-\timage.png\n3\t1\tmain.go\n",
			wantFiles: []diff.DiffFile{
				{Path: "image.png", IsBinary: true},
				{Path: "main.go", LinesAdded: 3, LinesRemoved: 1},
			},
			wantStats: diff.DiffStats{FilesChanged: 2, Additions: 3, Deletions: 1},
		},
		{
			name:      "whitespace only",
			input:     "\n\n",
			wantFiles: nil,
			wantStats: diff.DiffStats{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, stats := diff.ParseNumstat(tt.input)
			assert.Equal(t, tt.wantFiles, files)
			assert.Equal(t, tt.wantStats, stats)
		})
	}
}
