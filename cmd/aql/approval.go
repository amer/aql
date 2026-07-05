package main

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - approvalPrompt() — renders an ApprovalRequest into a y/n question
//   - isApproval() — parses the user's typed answer into a decision
//   - newApprover() — builds the tools.ApproverFn wired to the TUI
//
// MUST NOT GO HERE:
//   - The gate logic itself (internal/agent/tools/approval.go)
//   - TUI rendering of the prompt (reuses the ask_user modal)
//
// Q: Why y/n typed instead of a dedicated modal?
// A: It reuses the tested ask_user prompt path; a purpose-built approval
//    modal is a future refinement, not required for the gate to be safe.
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/amer/aql/internal/tui"
)

// approvalPrompt renders a human-readable y/n question for a gated tool call.
func approvalPrompt(req tools.ApprovalRequest) string {
	detail := approvalDetail(req)
	if detail != "" {
		return "Allow " + req.Tool + " to " + detail + "? (y/n)"
	}
	return "Allow " + req.Tool + "? (y/n)"
}

// approvalDetail extracts the salient argument (command / target path) from a
// gated tool's input so the user can judge the request. Unknown shapes yield "".
func approvalDetail(req tools.ApprovalRequest) string {
	var fields struct {
		Command  string `json:"command"`
		Path     string `json:"path"`
		FilePath string `json:"file_path"`
	}
	_ = json.Unmarshal(req.Input, &fields)
	switch {
	case fields.Command != "":
		return "run: " + fields.Command
	case fields.FilePath != "":
		return "edit " + fields.FilePath
	case fields.Path != "":
		return "write " + fields.Path
	default:
		return ""
	}
}

// isApproval reports whether a typed answer grants permission. Only an explicit
// yes approves; anything else (including empty or "n") denies.
func isApproval(answer string) bool {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// newApprover builds a tools.ApproverFn that prompts the TUI (via send) and
// blocks until the user answers or the context is cancelled.
func newApprover(send func(msg any)) tools.ApproverFn {
	return func(ctx context.Context, req tools.ApprovalRequest) (bool, error) {
		responseCh := make(chan string, 1)
		send(tui.AgentAskUserMsg{
			AgentName:  "coder",
			Question:   approvalPrompt(req),
			ResponseCh: responseCh,
		})
		select {
		case answer := <-responseCh:
			return isApproval(answer), nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
}
