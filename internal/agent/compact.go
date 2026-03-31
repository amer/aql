package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

const compactSystemPrompt = `You are a conversation summarizer. Summarize the following conversation concisely, preserving:
- Key decisions made
- Code, files, or architecture discussed or modified
- Open tasks or next steps
- Important context the assistant needs to continue helping

Be concise but thorough. Write in past tense as a factual summary.`

const compactMaxTokens = 4096

// FormatHistoryForCompaction converts a message history into readable text
// for summarization. Each message is prefixed with its role.
func FormatHistoryForCompaction(history []anthropic.MessageParam) string {
	if len(history) == 0 {
		return ""
	}

	var b strings.Builder
	for _, msg := range history {
		role := string(msg.Role)
		for _, block := range msg.Content {
			switch {
			case block.OfText != nil:
				prefix := "User"
				if role == "assistant" {
					prefix = "Assistant"
				}
				b.WriteString(fmt.Sprintf("%s: %s\n\n", prefix, block.OfText.Text))
			case block.OfToolUse != nil:
				b.WriteString(fmt.Sprintf("[Tool: %s]\n\n", block.OfToolUse.Name))
			case block.OfToolResult != nil:
				b.WriteString("[Tool Result]\n\n")
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// CompactHistory summarizes the conversation history via a Claude API call
// and replaces the full history with the summary.
func (a *Agent) CompactHistory(ctx context.Context) (string, error) {
	if len(a.history) < 2 {
		return "", fmt.Errorf("nothing to compact: conversation has fewer than 2 messages")
	}

	formatted := FormatHistoryForCompaction(a.history)

	model := ResolveModel(a.config.Model)

	slog.Debug("compacting conversation history",
		"agent", a.config.Name,
		"messages", len(a.history),
		"formatted_len", len(formatted),
	)

	resp, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: compactMaxTokens,
		System: []anthropic.TextBlockParam{
			{Text: compactSystemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(formatted)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("compact API call: %w", err)
	}

	var summary strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			summary.WriteString(block.Text)
		}
	}

	summaryText := summary.String()
	if summaryText == "" {
		return "", fmt.Errorf("compact produced empty summary")
	}

	// Replace history with compact summary
	a.history = []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Summary of prior conversation:\n\n" + summaryText)),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock("Understood. I have the context from our previous conversation. How can I help you next?")),
	}

	slog.Info("conversation compacted",
		"agent", a.config.Name,
		"summary_len", len(summaryText),
	)

	return summaryText, nil
}
