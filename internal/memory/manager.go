package memory

import (
	"path/filepath"
	"sort"
	"time"
)

// Manager coordinates all memory layers for a single agent.
type Manager struct {
	agentID   string
	shortTerm *ShortTerm
	shared    *Shared
	weights   ScorerWeights
}

// NewManager creates a memory manager for the given agent.
// The dir is used to store the shared memory file on disk.
func NewManager(agentID string, dir string) (*Manager, error) {
	shared, err := NewShared(filepath.Join(dir, agentID+"_shared.json"))
	if err != nil {
		return nil, err
	}

	return &Manager{
		agentID:   agentID,
		shortTerm: NewShortTerm(),
		shared:    shared,
		weights:   ScorerWeights{Alpha: 0.4, Beta: 0.2, Gamma: 0.4},
	}, nil
}

func (m *Manager) StoreShortTerm(entry Entry) error {
	return m.shortTerm.Put(entry)
}

func (m *Manager) GetShortTerm(id string) (Entry, error) {
	return m.shortTerm.Get(id)
}

func (m *Manager) StoreShared(entry Entry) error {
	return m.shared.Put(entry)
}

func (m *Manager) GetShared(id string) (Entry, error) {
	return m.shared.Get(id)
}

// Query retrieves the top-K most relevant entries across all memory layers,
// ranked by the combined recency/frequency/similarity score.
func (m *Manager) Query(queryEmbedding []float32, topK int) []Entry {
	now := time.Now()

	var all []Entry

	shortTermEntries, _ := m.shortTerm.List(m.agentID)
	all = append(all, shortTermEntries...)

	sharedEntries, _ := m.shared.List("")
	all = append(all, sharedEntries...)

	type scored struct {
		entry Entry
		score float64
	}

	ranked := make([]scored, len(all))
	for i, e := range all {
		ranked[i] = scored{entry: e, score: RelevanceScore(e, queryEmbedding, now, m.weights)}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	if topK > len(ranked) {
		topK = len(ranked)
	}

	result := make([]Entry, topK)
	for i := 0; i < topK; i++ {
		result[i] = ranked[i].entry
	}
	return result
}

// FlushShared persists shared memory to disk.
func (m *Manager) FlushShared() error {
	return m.shared.Flush()
}
