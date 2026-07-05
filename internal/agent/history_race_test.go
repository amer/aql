package agent_test

import (
	"sync"
	"testing"

	"github.com/amer/aql/internal/agent"
	"github.com/amer/aql/internal/domain"
	"github.com/stretchr/testify/require"
)

// TestAgent_ConcurrentHistoryMutation exercises the history mutators from
// several goroutines at once, mirroring what happens when a user hits /clear
// or /compact while a Run goroutine is still applying streamed history events.
// It must be run with -race to catch the data race it guards against.
func TestAgent_ConcurrentHistoryMutation(t *testing.T) {
	a, err := agent.New(agent.Config{Name: "t"}, t.TempDir(), testClientOpts("http://127.0.0.1:0")...)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(4)
		go func() { defer wg.Done(); a.ApplyHistory(domain.NewUserMessage("x")) }()
		go func() { defer wg.Done(); a.ReplaceHistory([]domain.Message{domain.NewUserMessage("y")}) }()
		go func() { defer wg.Done(); a.ClearHistory() }()
		go func() { defer wg.Done(); _ = a.HistoryLen() }()
	}
	wg.Wait()
}
