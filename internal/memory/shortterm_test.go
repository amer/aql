package memory_test

import (
	"testing"
	"time"

	"github.com/amer/aql/internal/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShortTermPutAndGet(t *testing.T) {
	store := memory.NewShortTerm()
	entry := memory.Entry{
		ID:        "e1",
		AgentID:   "coder",
		Content:   "implemented auth module",
		CreatedAt: time.Now(),
	}

	err := store.Put(entry)
	require.NoError(t, err)

	got, err := store.Get("e1")
	require.NoError(t, err)
	assert.Equal(t, "implemented auth module", got.Content)
	assert.Equal(t, "coder", got.AgentID)
}

func TestShortTermGetNotFound(t *testing.T) {
	store := memory.NewShortTerm()

	_, err := store.Get("nonexistent")
	assert.Error(t, err)
}

func TestShortTermDelete(t *testing.T) {
	store := memory.NewShortTerm()
	entry := memory.Entry{ID: "e1", AgentID: "coder", Content: "test"}

	require.NoError(t, store.Put(entry))
	require.NoError(t, store.Delete("e1"))

	_, err := store.Get("e1")
	assert.Error(t, err)
}

func TestShortTermDeleteNotFound(t *testing.T) {
	store := memory.NewShortTerm()
	err := store.Delete("nonexistent")
	assert.Error(t, err)
}

func TestShortTermListByAgent(t *testing.T) {
	store := memory.NewShortTerm()

	require.NoError(t, store.Put(memory.Entry{ID: "e1", AgentID: "coder", Content: "a"}))
	require.NoError(t, store.Put(memory.Entry{ID: "e2", AgentID: "reviewer", Content: "b"}))
	require.NoError(t, store.Put(memory.Entry{ID: "e3", AgentID: "coder", Content: "c"}))

	entries, err := store.List("coder")
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	for _, e := range entries {
		assert.Equal(t, "coder", e.AgentID)
	}
}

func TestShortTermListAll(t *testing.T) {
	store := memory.NewShortTerm()

	require.NoError(t, store.Put(memory.Entry{ID: "e1", AgentID: "coder", Content: "a"}))
	require.NoError(t, store.Put(memory.Entry{ID: "e2", AgentID: "reviewer", Content: "b"}))

	entries, err := store.List("")
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestShortTermPutOverwrite(t *testing.T) {
	store := memory.NewShortTerm()

	require.NoError(t, store.Put(memory.Entry{ID: "e1", AgentID: "coder", Content: "v1"}))
	require.NoError(t, store.Put(memory.Entry{ID: "e1", AgentID: "coder", Content: "v2"}))

	got, err := store.Get("e1")
	require.NoError(t, err)
	assert.Equal(t, "v2", got.Content)

	entries, err := store.List("")
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}
