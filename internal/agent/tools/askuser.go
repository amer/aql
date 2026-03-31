package tools

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - execAskUser — invokes the AskUserFn callback to prompt the user
//
// MUST NOT GO HERE:
//   - TUI rendering
//   - Direct I/O (stdin/stdout)
//   - Tool definitions
//
// Q: How does ask_user reach the TUI?
// A: main.go sets up the askFn callback closure that sends
//    AgentAskUserMsg to the TUI program. This file just calls that
//    callback.
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"encoding/json"
)

func execAskUser(ctx context.Context, input json.RawMessage, askFn AskUserFn) (string, error) {
	params, errMsg := parseInput[struct {
		Question string `json:"question"`
	}](input)
	if errMsg != "" {
		return errMsg, nil
	}
	if askFn == nil {
		return "ask_user is not available in this context", nil
	}
	q := UserQuestion{
		Question: params.Question,
		Response: make(chan string, 1),
	}
	answer, err := askFn(ctx, q)
	if err != nil {
		return "ask_user error: " + err.Error(), nil
	}
	return answer, nil
}
