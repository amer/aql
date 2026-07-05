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
// A: execBash already wraps ctx with bashTimeout as a backstop against a
//    command that never exits. Cancellation kills the whole process group.
//
// Q: Why does grep ignore the error?
// A: grep returns exit code 1 when no matches found — that's not an
//    error for our purposes.
//
// Q: Why the process group and WaitDelay dance in execBash?
// A: sh -c can background grandchildren that inherit the stdout pipe.
//    Killing only sh leaves them holding the pipe, so CombinedOutput would
//    block forever. Setpgid + a group-wide kill reaps the whole tree, and
//    WaitDelay force-closes the pipes if an orphan lingers.
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"encoding/json"
	"os/exec"
	"syscall"
	"time"
)

// bashTimeout bounds a single bash command. It is a backstop for commands that
// hang with no output; the ctx passed in may carry no deadline of its own.
const bashTimeout = 2 * time.Minute

// bashWaitDelay bounds how long Wait blocks after the process is signalled
// before its I/O pipes are force-closed, so a lingering orphan holding the pipe
// can't stall the tool indefinitely.
const bashWaitDelay = 2 * time.Second

func execBash(ctx context.Context, workDir string, input json.RawMessage) (string, error) {
	params, errMsg := parseInput[struct {
		Command string `json:"command"`
	}](input)
	if errMsg != "" {
		return errMsg, nil
	}

	ctx, cancel := context.WithTimeout(ctx, bashTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", params.Command)
	cmd.Dir = workDir
	// Run sh in its own process group so a single kill reaps backgrounded
	// grandchildren too — otherwise they hold the output pipe open.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		// Negative PID signals the whole process group.
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = bashWaitDelay

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
