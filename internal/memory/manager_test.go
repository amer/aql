package memory_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/amer/aql/internal/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager(t *testing.T) *memory.Manager {
	t.Helper()
	dir := t.TempDir()
	m, err := memory.NewManager("coder", dir)
	require.NoError(t, err)
	return m
}

func TestManagerStoreAndRetrieveFromShortTerm(t *testing.T) {
	m := newTestManager(t)

	entry := memory.Entry{
		ID:         "e1",
		AgentID:    "coder",
		Content:    "implemented auth",
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
		Embedding:  []float32{1, 0, 0},
	}

	require.NoError(t, m.StoreShortTerm(entry))

	got, err := m.GetShortTerm("e1")
	require.NoError(t, err)
	assert.Equal(t, "implemented auth", got.Content)
}

func TestManagerStoreAndRetrieveFromShared(t *testing.T) {
	m := newTestManager(t)

	entry := memory.Entry{
		ID:      "s1",
		AgentID: "coder",
		Content: "architecture decision: use event bus",
	}

	require.NoError(t, m.StoreShared(entry))

	got, err := m.GetShared("s1")
	require.NoError(t, err)
	assert.Equal(t, "architecture decision: use event bus", got.Content)
}

func TestManagerQueryRanksEntriesByRelevance(t *testing.T) {
	m := newTestManager(t)
	now := time.Now()

	relevant := memory.Entry{
		ID:          "e1",
		AgentID:     "coder",
		Content:     "auth module pattern",
		LastAccess:  now,
		AccessCount: 10,
		Embedding:   []float32{1, 0, 0},
	}
	irrelevant := memory.Entry{
		ID:          "e2",
		AgentID:     "coder",
		Content:     "old logging config",
		LastAccess:  now.Add(-90 * 24 * time.Hour),
		AccessCount: 1,
		Embedding:   []float32{0, 1, 0},
	}

	require.NoError(t, m.StoreShortTerm(relevant))
	require.NoError(t, m.StoreShortTerm(irrelevant))

	query := []float32{1, 0, 0}
	results := m.Query(query, 2)

	require.Len(t, results, 2)
	assert.Equal(t, "e1", results[0].ID, "most relevant entry should be first")
}

func TestManagerQueryRespectsTopK(t *testing.T) {
	m := newTestManager(t)
	now := time.Now()

	for i := 0; i < 5; i++ {
		require.NoError(t, m.StoreShortTerm(memory.Entry{
			ID:         fmt.Sprintf("e%d", i),
			AgentID:    "coder",
			Content:    "entry",
			LastAccess: now,
			Embedding:  []float32{1, 0, 0},
		}))
	}

	results := m.Query([]float32{1, 0, 0}, 3)
	assert.Len(t, results, 3)
}

func TestManagerQueryEmpty(t *testing.T) {
	m := newTestManager(t)

	results := m.Query([]float32{1, 0, 0}, 5)
	assert.Len(t, results, 0)
}
