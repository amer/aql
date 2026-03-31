package diff

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
)

// CommandRunner executes a command and returns its combined output.
// The default uses os/exec; tests inject a fake.
type CommandRunner func(ctx context.Context, workDir, name string, args ...string) ([]byte, error)

// Runner executes git diff commands and returns structured results.
type Runner struct {
	run CommandRunner
}

// NewRunner creates a Runner with the given command executor.
func NewRunner(run CommandRunner) *Runner {
	return &Runner{run: run}
}

// NewDefaultRunner creates a Runner that shells out to real git.
func NewDefaultRunner() *Runner {
	return NewRunner(execCommand)
}

// Run executes `git diff HEAD --numstat` and `git diff HEAD` in workDir,
// parses both outputs, and merges hunks into the file list.
func (r *Runner) Run(ctx context.Context, workDir string) ([]DiffFile, DiffStats, error) {
	slog.Debug("running git diff", "workDir", workDir)

	numstatOut, err := r.run(ctx, workDir, "git", "diff", "HEAD", "--numstat")
	if err != nil {
		return nil, DiffStats{}, fmt.Errorf("git diff --numstat: %w", err)
	}

	files, stats := ParseNumstat(string(numstatOut))
	if len(files) == 0 {
		return nil, DiffStats{}, nil
	}

	diffOut, err := r.run(ctx, workDir, "git", "diff", "HEAD")
	if err != nil {
		return nil, DiffStats{}, fmt.Errorf("git diff: %w", err)
	}

	parsed := ParseUnifiedDiff(string(diffOut))
	mergeHunks(files, parsed)

	slog.Debug("git diff complete", "files", len(files), "additions", stats.Additions, "deletions", stats.Deletions)
	return files, stats, nil
}

// mergeHunks attaches parsed hunks to the corresponding files from numstat.
func mergeHunks(files []DiffFile, parsed []DiffFile) {
	byPath := make(map[string][]DiffHunk, len(parsed))
	for _, f := range parsed {
		byPath[f.Path] = f.Hunks
	}
	for i := range files {
		if hunks, ok := byPath[files[i].Path]; ok {
			files[i].Hunks = hunks
		}
	}
}

// execCommand is the default CommandRunner using os/exec.
func execCommand(ctx context.Context, workDir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workDir
	return cmd.CombinedOutput()
}
