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

func TestManagerQueryLargeDataset(t *testing.T) {
	m := newTestManager(t)
	now := time.Now()

	// Insert 1000 entries with varying relevance
	for i := 0; i < 1000; i++ {
		emb := []float32{float32(i % 10), float32(i % 7), float32(i % 3)}
		require.NoError(t, m.StoreShortTerm(memory.Entry{
			ID:          fmt.Sprintf("e%d", i),
			AgentID:     "coder",
			Content:     fmt.Sprintf("entry %d", i),
			LastAccess:  now.Add(-time.Duration(i) * time.Hour),
			AccessCount: i%20 + 1,
			Embedding:   emb,
		}))
	}

	results := m.Query([]float32{9, 6, 2}, 5)
	require.Len(t, results, 5)

	// Results should be sorted by descending score
	for i := 1; i < len(results); i++ {
		scoreA := memory.RelevanceScore(results[i-1], []float32{9, 6, 2}, now, memory.ScorerWeights{Alpha: 0.4, Beta: 0.2, Gamma: 0.4})
		scoreB := memory.RelevanceScore(results[i], []float32{9, 6, 2}, now, memory.ScorerWeights{Alpha: 0.4, Beta: 0.2, Gamma: 0.4})
		assert.GreaterOrEqual(t, scoreA, scoreB, "results should be sorted by score")
	}
}

func TestManagerQueryParallelCorrectness(t *testing.T) {
	m := newTestManager(t)
	now := time.Now()

	// The best entry has the highest recency + similarity
	best := memory.Entry{
		ID:          "best",
		AgentID:     "coder",
		Content:     "most relevant",
		LastAccess:  now,
		AccessCount: 100,
		Embedding:   []float32{1, 0, 0},
	}
	require.NoError(t, m.StoreShortTerm(best))

	// Add many lower-scoring entries
	for i := 0; i < 500; i++ {
		require.NoError(t, m.StoreShortTerm(memory.Entry{
			ID:          fmt.Sprintf("e%d", i),
			AgentID:     "coder",
			Content:     "noise",
			LastAccess:  now.Add(-time.Duration(30+i) * 24 * time.Hour),
			AccessCount: 1,
			Embedding:   []float32{0, 1, 0},
		}))
	}

	results := m.Query([]float32{1, 0, 0}, 1)
	require.Len(t, results, 1)
	assert.Equal(t, "best", results[0].ID, "parallel scoring should still find the best entry")
}

func BenchmarkManagerQuery(b *testing.B) {
	dir := b.TempDir()
	m, err := memory.NewManager("coder", dir)
	require.NoError(b, err)
	now := time.Now()

	for i := 0; i < 10000; i++ {
		_ = m.StoreShortTerm(memory.Entry{
			ID:          fmt.Sprintf("e%d", i),
			AgentID:     "coder",
			Content:     fmt.Sprintf("entry %d", i),
			LastAccess:  now.Add(-time.Duration(i) * time.Hour),
			AccessCount: i%20 + 1,
			Embedding:   []float32{float32(i % 10), float32(i % 7), float32(i % 3)},
		})
	}

	query := []float32{9, 6, 2}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Query(query, 10)
	}
}

func TestManagerQueryTopKHeap(t *testing.T) {
	m := newTestManager(t)
	now := time.Now()

	// Create entries where we know the exact ranking
	for i := 0; i < 20; i++ {
		require.NoError(t, m.StoreShortTerm(memory.Entry{
			ID:          fmt.Sprintf("e%d", i),
			AgentID:     "coder",
			Content:     fmt.Sprintf("entry %d", i),
			LastAccess:  now,
			AccessCount: i + 1, // higher i = higher frequency score
			Embedding:   []float32{1, 0, 0},
		}))
	}

	results := m.Query([]float32{1, 0, 0}, 3)
	require.Len(t, results, 3)

	// Top 3 should be e19, e18, e17 (highest access counts)
	assert.Equal(t, "e19", results[0].ID)
	assert.Equal(t, "e18", results[1].ID)
	assert.Equal(t, "e17", results[2].ID)
}
