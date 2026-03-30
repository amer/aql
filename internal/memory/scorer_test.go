package memory_test

import (
	"testing"
	"time"

	"github.com/amer/aql/internal/memory"
	"github.com/stretchr/testify/assert"
)

func TestRecencyScore(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		lastAccess time.Time
		wantHigh   bool
	}{
		{"just accessed", now, true},
		{"accessed 1 hour ago", now.Add(-1 * time.Hour), true},
		{"accessed 30 days ago", now.Add(-30 * 24 * time.Hour), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := memory.RecencyScore(tt.lastAccess, now)
			assert.InDelta(t, 1.0, score+1.0, 2.0, "score should be between 0 and 1")
			if tt.wantHigh {
				assert.Greater(t, score, 0.5)
			} else {
				assert.Less(t, score, 0.5)
			}
		})
	}
}

func TestFrequencyScore(t *testing.T) {
	tests := []struct {
		name        string
		accessCount int
		want        float64
	}{
		{"never accessed", 0, 0.0},
		{"accessed once", 1, 0.0},
		{"accessed 10 times", 10, 1.0},
		{"accessed 100 times", 100, 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := memory.FrequencyScore(tt.accessCount)
			assert.InDelta(t, tt.want, score, 0.01)
		})
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a    []float32
		b    []float32
		want float64
	}{
		{"identical vectors", []float32{1, 0, 0}, []float32{1, 0, 0}, 1.0},
		{"orthogonal vectors", []float32{1, 0, 0}, []float32{0, 1, 0}, 0.0},
		{"opposite vectors", []float32{1, 0, 0}, []float32{-1, 0, 0}, -1.0},
		{"similar vectors", []float32{1, 1, 0}, []float32{1, 0, 0}, 0.707},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := memory.CosineSimilarity(tt.a, tt.b)
			assert.InDelta(t, tt.want, score, 0.01)
		})
	}
}

func TestCosineSimilarityEdgeCases(t *testing.T) {
	assert.Equal(t, 0.0, memory.CosineSimilarity(nil, nil))
	assert.Equal(t, 0.0, memory.CosineSimilarity([]float32{0, 0, 0}, []float32{0, 0, 0}))
	assert.Equal(t, 0.0, memory.CosineSimilarity([]float32{1, 2}, []float32{1, 2, 3}))
}

func TestRelevanceScore(t *testing.T) {
	now := time.Now()
	query := []float32{1, 0, 0}

	recent := memory.Entry{
		LastAccess:  now,
		AccessCount: 10,
		Embedding:   []float32{1, 0, 0},
	}

	stale := memory.Entry{
		LastAccess:  now.Add(-60 * 24 * time.Hour),
		AccessCount: 1,
		Embedding:   []float32{0, 1, 0},
	}

	weights := memory.ScorerWeights{Alpha: 0.4, Beta: 0.2, Gamma: 0.4}

	recentScore := memory.RelevanceScore(recent, query, now, weights)
	staleScore := memory.RelevanceScore(stale, query, now, weights)

	assert.Greater(t, recentScore, staleScore, "recent relevant entry should score higher than stale irrelevant one")
	assert.Greater(t, recentScore, 0.0)
}
