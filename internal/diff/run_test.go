package diff_test

import (
	"context"
	"errors"
	"testing"

	"github.com/amer/aql/internal/diff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_Run(t *testing.T) {
	numstatOutput := "5\t2\tREADME.md\n3\t1\tmain.go\n"
	unifiedOutput := "diff --git a/README.md b/README.md\n" +
		"index abc..def 100644\n" +
		"--- a/README.md\n" +
		"+++ b/README.md\n" +
		"@@ -1,3 +1,6 @@\n" +
		" # Project\n" +
		"+\n" +
		"+New section.\n" +
		" \n" +
		" Old content.\n" +
		"+More content.\n" +
		" End.\n" +
		"diff --git a/main.go b/main.go\n" +
		"index abc..def 100644\n" +
		"--- a/main.go\n" +
		"+++ b/main.go\n" +
		"@@ -1,2 +1,4 @@\n" +
		" package main\n" +
		"+\n" +
		"+import \"fmt\"\n" +
		"-// old comment\n"

	callCount := 0
	runner := diff.NewRunner(func(ctx context.Context, workDir, name string, args ...string) ([]byte, error) {
		callCount++
		if callCount == 1 {
			// First call: git diff HEAD --numstat
			assert.Equal(t, "git", name)
			assert.Contains(t, args, "--numstat")
			return []byte(numstatOutput), nil
		}
		// Second call: git diff HEAD
		assert.Equal(t, "git", name)
		assert.NotContains(t, args, "--numstat")
		return []byte(unifiedOutput), nil
	})

	files, stats, err := runner.Run(context.Background(), "/tmp/repo")
	require.NoError(t, err)

	assert.Equal(t, 2, stats.FilesChanged)
	assert.Equal(t, 8, stats.Additions)
	assert.Equal(t, 3, stats.Deletions)
	require.Len(t, files, 2)

	assert.Equal(t, "README.md", files[0].Path)
	assert.Equal(t, 5, files[0].LinesAdded)
	assert.Equal(t, 2, files[0].LinesRemoved)
	require.Len(t, files[0].Hunks, 1)
	assert.Equal(t, 7, len(files[0].Hunks[0].Lines))

	assert.Equal(t, "main.go", files[1].Path)
	assert.Equal(t, 3, files[1].LinesAdded)
	assert.Equal(t, 1, files[1].LinesRemoved)
	require.Len(t, files[1].Hunks, 1)
}

func TestRunner_Run_numstat_error(t *testing.T) {
	runner := diff.NewRunner(func(ctx context.Context, workDir, name string, args ...string) ([]byte, error) {
		return nil, errors.New("git not found")
	})

	_, _, err := runner.Run(context.Background(), "/tmp/repo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git not found")
}

func TestRunner_Run_empty_diff(t *testing.T) {
	runner := diff.NewRunner(func(ctx context.Context, workDir, name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	})

	files, stats, err := runner.Run(context.Background(), "/tmp/repo")
	require.NoError(t, err)
	assert.Empty(t, files)
	assert.Equal(t, diff.DiffStats{}, stats)
}
