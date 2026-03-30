package agent_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amer/aql/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadClaudeMDFromDir(t *testing.T) {
	dir := t.TempDir()
	content := "# Rules\n- Use TDD\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(content), 0644))

	result, err := agent.LoadClaudeMD(dir)
	require.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestLoadClaudeMDNotFound(t *testing.T) {
	dir := t.TempDir()

	result, err := agent.LoadClaudeMD(dir)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestCollectClaudeMDFiles(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.MkdirAll(sub, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("root rules\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "CLAUDE.md"), []byte("sub rules\n"), 0644))

	result := agent.CollectClaudeMD(root, sub)
	assert.Contains(t, result, "root rules")
	assert.Contains(t, result, "sub rules")
}

func TestCollectClaudeMDNoneExist(t *testing.T) {
	dir := t.TempDir()

	result := agent.CollectClaudeMD(dir)
	assert.Equal(t, "", result)
}

func TestCollectClaudeMDDeduplicates(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("rules\n"), 0644))

	result := agent.CollectClaudeMD(dir, dir)
	assert.Equal(t, 1, countOccurrences(result, "rules"))
}

func countOccurrences(s, sub string) int {
	count := 0
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			count++
		}
	}
	return count
}
