package diff_test

import (
	"testing"

	"github.com/amer/aql/internal/diff"
	"github.com/stretchr/testify/assert"
)

func TestParseUnifiedDiff(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantFiles []diff.DiffFile
	}{
		{
			name:      "empty output",
			input:     "",
			wantFiles: nil,
		},
		{
			name: "single file single hunk",
			input: "diff --git a/main.go b/main.go\n" +
				"index abc1234..def5678 100644\n" +
				"--- a/main.go\n" +
				"+++ b/main.go\n" +
				"@@ -1,3 +1,4 @@\n" +
				" package main\n" +
				" \n" +
				"+import \"fmt\"\n" +
				" func main() {}\n",
			wantFiles: []diff.DiffFile{
				{
					Path: "main.go",
					Hunks: []diff.DiffHunk{
						{
							OldStart: 1, OldCount: 3,
							NewStart: 1, NewCount: 4,
							Lines: []diff.DiffLine{
								{Type: diff.DiffContext, Content: "package main"},
								{Type: diff.DiffContext, Content: ""},
								{Type: diff.DiffAdded, Content: "import \"fmt\""},
								{Type: diff.DiffContext, Content: "func main() {}"},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple hunks",
			input: `diff --git a/file.go b/file.go
index abc..def 100644
--- a/file.go
+++ b/file.go
@@ -1,3 +1,3 @@
 line1
-old2
+new2
 line3
@@ -10,2 +10,3 @@
 line10
+added11
 line11
`,
			wantFiles: []diff.DiffFile{
				{
					Path: "file.go",
					Hunks: []diff.DiffHunk{
						{
							OldStart: 1, OldCount: 3,
							NewStart: 1, NewCount: 3,
							Lines: []diff.DiffLine{
								{Type: diff.DiffContext, Content: "line1"},
								{Type: diff.DiffRemoved, Content: "old2"},
								{Type: diff.DiffAdded, Content: "new2"},
								{Type: diff.DiffContext, Content: "line3"},
							},
						},
						{
							OldStart: 10, OldCount: 2,
							NewStart: 10, NewCount: 3,
							Lines: []diff.DiffLine{
								{Type: diff.DiffContext, Content: "line10"},
								{Type: diff.DiffAdded, Content: "added11"},
								{Type: diff.DiffContext, Content: "line11"},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple files",
			input: `diff --git a/a.go b/a.go
index abc..def 100644
--- a/a.go
+++ b/a.go
@@ -1,2 +1,3 @@
 package a
+// comment
 func A() {}
diff --git a/b.go b/b.go
index abc..def 100644
--- a/b.go
+++ b/b.go
@@ -1,2 +1,2 @@
 package b
-func Old() {}
+func New() {}
`,
			wantFiles: []diff.DiffFile{
				{
					Path: "a.go",
					Hunks: []diff.DiffHunk{
						{
							OldStart: 1, OldCount: 2,
							NewStart: 1, NewCount: 3,
							Lines: []diff.DiffLine{
								{Type: diff.DiffContext, Content: "package a"},
								{Type: diff.DiffAdded, Content: "// comment"},
								{Type: diff.DiffContext, Content: "func A() {}"},
							},
						},
					},
				},
				{
					Path: "b.go",
					Hunks: []diff.DiffHunk{
						{
							OldStart: 1, OldCount: 2,
							NewStart: 1, NewCount: 2,
							Lines: []diff.DiffLine{
								{Type: diff.DiffContext, Content: "package b"},
								{Type: diff.DiffRemoved, Content: "func Old() {}"},
								{Type: diff.DiffAdded, Content: "func New() {}"},
							},
						},
					},
				},
			},
		},
		{
			name: "new file",
			input: "diff --git a/new.go b/new.go\n" +
				"new file mode 100644\n" +
				"index 0000000..abc1234\n" +
				"--- /dev/null\n" +
				"+++ b/new.go\n" +
				"@@ -0,0 +1,3 @@\n" +
				"+package new\n" +
				"+\n" +
				"+func Hello() {}\n",
			wantFiles: []diff.DiffFile{
				{
					Path: "new.go",
					Hunks: []diff.DiffHunk{
						{
							OldStart: 0, OldCount: 0,
							NewStart: 1, NewCount: 3,
							Lines: []diff.DiffLine{
								{Type: diff.DiffAdded, Content: "package new"},
								{Type: diff.DiffAdded, Content: ""},
								{Type: diff.DiffAdded, Content: "func Hello() {}"},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := diff.ParseUnifiedDiff(tt.input)
			assert.Equal(t, tt.wantFiles, files)
		})
	}
}

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
