package tools

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - ApprovalRequest, ApproverFn — the approval port types
//   - requiresApproval() — which tools are gated before execution
//   - gate() — wraps an ExecutorFn with the approval check
//
// MUST NOT GO HERE:
//   - The TUI prompt itself (main.go wires an ApproverFn that talks to the TUI)
//   - Individual tool logic (each tool has its own file)
//
// Q: Why gate here and not inside each tool?
// A: Approval is a cross-cutting policy, not tool logic. Gating in one
//    place keeps the guarded-tool list authoritative and testable.
//
// Q: What happens with no ApproverFn?
// A: Tools run ungated — backward compatible. main.go injects a real
//    approver for interactive sessions; tests opt in via WithApprover.
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"encoding/json"
	"log/slog"
)

// ApprovalRequest describes a side-effecting tool call awaiting user consent.
type ApprovalRequest struct {
	Tool    string
	WorkDir string
	Input   json.RawMessage
}

// ApproverFn decides whether a side-effecting tool call may proceed. Returning
// (false, nil) denies the call; a non-nil error is an infrastructure failure
// (e.g. the prompt channel was torn down).
type ApproverFn func(ctx context.Context, req ApprovalRequest) (bool, error)

// guardedTools are the side-effecting tools that require user approval before
// they run. Read-only tools (read_file, glob, grep, list_directory, web_search,
// web_fetch) are not gated — web_fetch is untrusted-input, not side-effecting,
// and is constrained separately.
var guardedTools = map[string]bool{
	"bash":          true,
	"write_file":    true,
	"edit":          true,
	"notebook_edit": true,
}

// requiresApproval reports whether a tool must be approved before execution.
func requiresApproval(name string) bool {
	return guardedTools[name]
}

// gate wraps next with an approval check for guarded tools. When approve is nil,
// next runs unchanged. A denied call returns a tool-error string (nil Go error),
// per the tool-error convention, so the model sees the denial and can adapt.
func gate(approve ApproverFn, next ExecutorFn) ExecutorFn {
	if approve == nil {
		return next
	}
	return func(ctx context.Context, workDir, name string, input json.RawMessage) (string, error) {
		if !requiresApproval(name) {
			return next(ctx, workDir, name, input)
		}
		ok, err := approve(ctx, ApprovalRequest{Tool: name, WorkDir: workDir, Input: input})
		if err != nil {
			return "", err
		}
		if !ok {
			slog.Info("tool call denied by user", "tool", name, "workDir", workDir)
			return "tool call to " + name + " was denied by the user", nil
		}
		return next(ctx, workDir, name, input)
	}
}
