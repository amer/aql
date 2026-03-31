package tools

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Shell execution handlers — execBash (runs sh -c), execGrep
//     (runs grep with flags)
//   - Both take full (ctx, workDir, json.RawMessage) signature
//
// MUST NOT GO HERE:
//   - File I/O tools (file.go)
//   - Tool definitions (defs.go)
//   - Output formatting for TUI
//
// Q: Should I add a timeout to bash?
// A: The ctx already carries a timeout. Bash execution respects
//    context cancellation.
//
// Q: Why does grep ignore the error?
// A: grep returns exit code 1 when no matches found — that's not an
//    error for our purposes.
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"encoding/json"
	"os/exec"
)

func execBash(ctx context.Context, workDir string, input json.RawMessage) (string, error) {
	params, errMsg := parseInput[struct {
		Command string `json:"command"`
	}](input)
	if errMsg != "" {
		return errMsg, nil
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", params.Command)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	result := string(out)
	if err != nil {
		result += "\nexit: " + err.Error()
	}
	return result, nil
}

func execGrep(ctx context.Context, workDir string, input json.RawMessage) (string, error) {
	params, errMsg := parseInput[struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
		Include string `json:"include"`
	}](input)
	if errMsg != "" {
		return errMsg, nil
	}
	searchPath := workDir
	if params.Path != "" {
		searchPath = resolvePath(workDir, params.Path)
	}
	args := []string{"-rn", params.Pattern, searchPath}
	if params.Include != "" {
		args = []string{"-rn", "--include=" + params.Include, params.Pattern, searchPath}
	}
	cmd := exec.CommandContext(ctx, "grep", args...)
	cmd.Dir = workDir
	out, _ := cmd.CombinedOutput()
	result := string(out)
	if len(result) > 10000 {
		result = result[:10000] + "\n... (truncated)"
	}
	return result, nil
}
