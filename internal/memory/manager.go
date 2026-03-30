package memory

import (
	"container/heap"
	"log/slog"
	"path/filepath"
	"runtime"
	"sync"
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
	slog.Debug("initializing memory manager", "agentID", agentID, "dir", dir)

	shared, err := NewShared(filepath.Join(dir, agentID+"_shared.json"))
	if err != nil {
		slog.Error("failed to init shared memory", "agentID", agentID, "error", err)
		return nil, err
	}

	slog.Debug("memory manager ready", "agentID", agentID)
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

// scored pairs an entry with its relevance score.
type scored struct {
	entry Entry
	score float64
}

// scoredHeap is a min-heap of scored entries, used for top-K selection.
type scoredHeap []scored

func (h scoredHeap) Len() int           { return len(h) }
func (h scoredHeap) Less(i, j int) bool { return h[i].score < h[j].score } // min-heap
func (h scoredHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *scoredHeap) Push(x any)        { *h = append(*h, x.(scored)) }
func (h *scoredHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// Query retrieves the top-K most relevant entries across all memory layers,
// ranked by the combined recency/frequency/similarity score.
// Uses parallel scoring and a min-heap for O(n log k) selection.
func (m *Manager) Query(queryEmbedding []float32, topK int) []Entry {
	now := time.Now()

	var all []Entry

	shortTermEntries, _ := m.shortTerm.List(m.agentID)
	all = append(all, shortTermEntries...)

	sharedEntries, _ := m.shared.List("")
	all = append(all, sharedEntries...)

	slog.Debug("memory query", "agentID", m.agentID, "topK", topK, "shortTerm", len(shortTermEntries), "shared", len(sharedEntries))

	n := len(all)
	if n == 0 {
		return nil
	}
	if topK > n {
		topK = n
	}

	// Score all entries in parallel using worker pool
	scores := make([]float64, n)
	workers := runtime.NumCPU()
	if workers > n {
		workers = n
	}

	var wg sync.WaitGroup
	chunkSize := (n + workers - 1) / workers
	for w := 0; w < workers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > n {
			end = n
		}
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for i := start; i < end; i++ {
				scores[i] = RelevanceScore(all[i], queryEmbedding, now, m.weights)
			}
		}(start, end)
	}
	wg.Wait()

	// Use a min-heap of size k for O(n log k) top-K selection
	h := make(scoredHeap, 0, topK)
	heap.Init(&h)

	for i := 0; i < n; i++ {
		if h.Len() < topK {
			heap.Push(&h, scored{entry: all[i], score: scores[i]})
		} else if scores[i] > h[0].score {
			h[0] = scored{entry: all[i], score: scores[i]}
			heap.Fix(&h, 0)
		}
	}

	// Extract from heap in descending score order
	result := make([]Entry, h.Len())
	for i := len(result) - 1; i >= 0; i-- {
		result[i] = heap.Pop(&h).(scored).entry
	}

	if len(result) > 0 {
		slog.Debug("memory query results", "agentID", m.agentID, "returned", len(result), "topScore", scores[0])
	}
	return result
}

// FlushShared persists shared memory to disk.
func (m *Manager) FlushShared() error {
	return m.shared.Flush()
}
