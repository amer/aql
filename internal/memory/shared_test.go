package memory_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amer/aql/internal/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSharedPutAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := memory.NewShared(filepath.Join(dir, "shared.json"))
	require.NoError(t, err)

	entry := memory.Entry{
		ID:        "s1",
		AgentID:   "coder",
		Content:   "shared context",
		CreatedAt: time.Now(),
	}

	require.NoError(t, store.Put(entry))

	got, err := store.Get("s1")
	require.NoError(t, err)
	assert.Equal(t, "shared context", got.Content)
}

func TestSharedPersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shared.json")

	store1, err := memory.NewShared(path)
	require.NoError(t, err)
	require.NoError(t, store1.Put(memory.Entry{ID: "s1", AgentID: "coder", Content: "persisted"}))
	require.NoError(t, store1.Flush())

	_, err = os.Stat(path)
	require.NoError(t, err, "file should exist on disk")

	store2, err := memory.NewShared(path)
	require.NoError(t, err)

	got, err := store2.Get("s1")
	require.NoError(t, err)
	assert.Equal(t, "persisted", got.Content)
}

func TestSharedDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := memory.NewShared(filepath.Join(dir, "shared.json"))
	require.NoError(t, err)

	require.NoError(t, store.Put(memory.Entry{ID: "s1", AgentID: "coder", Content: "x"}))
	require.NoError(t, store.Delete("s1"))

	_, err = store.Get("s1")
	assert.Error(t, err)
}

func TestSharedListByAgent(t *testing.T) {
	dir := t.TempDir()
	store, err := memory.NewShared(filepath.Join(dir, "shared.json"))
	require.NoError(t, err)

	require.NoError(t, store.Put(memory.Entry{ID: "s1", AgentID: "coder", Content: "a"}))
	require.NoError(t, store.Put(memory.Entry{ID: "s2", AgentID: "reviewer", Content: "b"}))
	require.NoError(t, store.Put(memory.Entry{ID: "s3", AgentID: "coder", Content: "c"}))

	entries, err := store.List("coder")
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestSharedNewFromEmptyPath(t *testing.T) {
	dir := t.TempDir()
	store, err := memory.NewShared(filepath.Join(dir, "new.json"))
	require.NoError(t, err)

	entries, err := store.List("")
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}
