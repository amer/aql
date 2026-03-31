package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/amer/aql/internal/domain"
	"github.com/amer/aql/internal/models"
)

const compactSystemPrompt = `You are a conversation summarizer. Summarize the following conversation concisely, preserving:
- Key decisions made
- Code, files, or architecture discussed or modified
- Open tasks or next steps
- Important context the assistant needs to continue helping

Be concise but thorough. Write in past tense as a factual summary.`

const compactMaxTokens = 4096

// AutoCompactThreshold is the input token count above which auto-compaction triggers.
// Set to 80% of a typical 200k context window.
const AutoCompactThreshold = 160_000

// FormatHistoryForCompaction converts a message history into readable text
// for summarization. Each message is prefixed with its role.
func FormatHistoryForCompaction(history []domain.Message) string {
	if len(history) == 0 {
		return ""
	}

	var b strings.Builder
	for _, msg := range history {
		for _, block := range msg.Content {
			switch {
			case block.ToolUse != nil:
				b.WriteString(fmt.Sprintf("[Tool: %s]\n\n", block.ToolUse.Name))
			case block.ToolResult != nil:
				b.WriteString("[Tool Result]\n\n")
			default:
				prefix := "User"
				if msg.Role == domain.RoleAssistant {
					prefix = "Assistant"
				}
				b.WriteString(fmt.Sprintf("%s: %s\n\n", prefix, block.Text))
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// CompactHistory summarizes the conversation history via a Claude API call
// and replaces the full history with the summary.
func (a *Agent) CompactHistory(ctx context.Context) (string, error) {
	summary, compacted, err := a.summarizeHistory(ctx, a.history)
	if err != nil {
		return "", err
	}
	a.history = compacted
	return summary, nil
}

// summarizeHistory performs the API call to summarize history and returns
// the summary text and the replacement history messages. Does not mutate
// any agent state, so it's safe to call from any goroutine.
func (a *Agent) summarizeHistory(ctx context.Context, history []domain.Message) (string, []domain.Message, error) {
	if len(history) < 2 {
		return "", nil, fmt.Errorf("nothing to compact: conversation has fewer than 2 messages")
	}

	formatted := FormatHistoryForCompaction(history)
	model := models.ResolveModel(a.config.Model)

	slog.Debug("compacting conversation history",
		"agent", a.config.Name,
		"messages", len(history),
		"formatted_len", len(formatted),
	)

	resp, err := a.chatClient.SendMessage(ctx, domain.ChatParams{
		Model:     model,
		MaxTokens: compactMaxTokens,
		System:    compactSystemPrompt,
		Messages:  []domain.Message{domain.NewUserMessage(formatted)},
	})
	if err != nil {
		return "", nil, fmt.Errorf("compact API call: %w", err)
	}

	summaryText := strings.Join(resp.TextParts, "")
	if summaryText == "" {
		return "", nil, fmt.Errorf("compact produced empty summary")
	}

	compacted := []domain.Message{
		domain.NewUserMessage("Summary of prior conversation:\n\n" + summaryText),
		domain.NewAssistantMessage("Understood. I have the context from our previous conversation. How can I help you next?"),
	}

	slog.Info("conversation compacted",
		"agent", a.config.Name,
		"summary_len", len(summaryText),
	)

	return summaryText, compacted, nil
}
