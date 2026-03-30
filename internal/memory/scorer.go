package memory

import (
	"math"
	"time"
)

// ScorerWeights controls the balance between recency, frequency, and similarity.
type ScorerWeights struct {
	Alpha float64 // recency weight
	Beta  float64 // frequency weight
	Gamma float64 // similarity weight
}

// RecencyScore returns a score between 0 and 1 based on how recently
// the entry was accessed. Uses exponential decay with a half-life of 7 days.
func RecencyScore(lastAccess, now time.Time) float64 {
	hoursSince := now.Sub(lastAccess).Hours()
	if hoursSince < 0 {
		hoursSince = 0
	}
	halfLifeHours := 7.0 * 24.0 // 7 days
	return math.Exp(-math.Ln2 * hoursSince / halfLifeHours)
}

// FrequencyScore returns a log-scaled score based on access count.
// Returns 0 for counts <= 1, otherwise log10(count).
func FrequencyScore(accessCount int) float64 {
	if accessCount <= 1 {
		return 0.0
	}
	return math.Log10(float64(accessCount))
}

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns 0 for zero-length, mismatched, or zero-magnitude vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0.0
	}

	var dot, magA, magB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		magA += float64(a[i]) * float64(a[i])
		magB += float64(b[i]) * float64(b[i])
	}

	mag := math.Sqrt(magA) * math.Sqrt(magB)
	if mag == 0 {
		return 0.0
	}
	return dot / mag
}

// RelevanceScore computes the combined relevance score for an entry
// given a query embedding and the current time.
func RelevanceScore(entry Entry, queryEmbedding []float32, now time.Time, w ScorerWeights) float64 {
	r := RecencyScore(entry.LastAccess, now)
	f := FrequencyScore(entry.AccessCount)
	s := CosineSimilarity(entry.Embedding, queryEmbedding)

	// Normalize similarity from [-1,1] to [0,1]
	sNorm := (s + 1.0) / 2.0

	return w.Alpha*r + w.Beta*f + w.Gamma*sNorm
}
