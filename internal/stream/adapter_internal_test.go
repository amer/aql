package stream

// Internal test: drainHistory is an unexported helper on the cancellation path.
// It is tested directly (rather than only through Forward*) so the "apply every
// remaining history mutation" guarantee is verified deterministically, without
// depending on select scheduling.

import (
	"testing"

	"github.com/amer/aql/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestDrainHistory_AppliesAllRemainingHistory(t *testing.T) {
	ch := make(chan domain.StreamEvent, 4)
	ch <- domain.StreamEvent{History: &domain.HistoryAppendMsg{Message: domain.NewUserMessage("u")}}
	ch <- domain.StreamEvent{History: &domain.HistoryAppendMsg{Message: domain.Message{Role: domain.RoleAssistant}}}
	ch <- domain.StreamEvent{Replace: &domain.HistoryReplaceMsg{Messages: []domain.Message{domain.NewUserMessage("s")}}}
	ch <- domain.StreamEvent{Done: true}
	close(ch)

	var appended []domain.Message
	var replaced [][]domain.Message
	drainHistory(ch, HistoryCallbacks{
		Append:  func(m domain.Message) { appended = append(appended, m) },
		Replace: func(ms []domain.Message) { replaced = append(replaced, ms) },
	})

	assert.Len(t, appended, 2, "both HistoryAppend events must be applied")
	assert.Len(t, replaced, 1, "the HistoryReplace event must be applied")
}
