package agent

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - EnvironmentInfo() — formats env context for system prompt,
//     GitStatus() — short git status, CheckEnv() — API key validation,
//     shell detection, git helper functions (isGitRepo, gitCommand).
//
// MUST NOT GO HERE:
//   - System prompt assembly (agent.go)
//   - Authentication logic (auth/)
//   - Model resolution (models/)
//   - Anything that imports other internal packages
//
// Q: Should I add more environment info to the system prompt?
// A: Add it to EnvironmentInfo() here. It returns a formatted string
//    block.
//
// Q: Can I use this for runtime git operations?
// A: No. This is for system prompt context only. Runtime git ops belong
//    in tools.
// ──────────────────────────────────────────────────────────────────

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// CheckEnv validates that the API key is set and non-empty.
func CheckEnv(apiKey string) error {
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY is not set\n\n  export ANTHROPIC_API_KEY=<your-key>")
	}
	return nil
}

// EnvironmentInfo returns a formatted block of environment context for the system prompt.
func EnvironmentInfo(workDir string, modelID string) string {
	abs, _ := filepath.Abs(workDir)
	if abs == "" {
		abs = workDir
	}

	gitRepo := isGitRepo(workDir)
	gitRepoStr := "no"
	if gitRepo {
		gitRepoStr = "yes"
	}

	shell := detectShell()
	date := time.Now().Format("2006-01-02")

	return fmt.Sprintf(`# Environment
- Date: %s
- Working directory: %s
- Platform: %s/%s
- Shell: %s
- Git repo: %s
- Model: %s`, date, abs, runtime.GOOS, runtime.GOARCH, shell, gitRepoStr, modelID)
}

// GitStatus returns a short git status summary for the working directory.
// Returns empty string if not a git repo or on error.
func GitStatus(dir string) string {
	branch := gitCommand(dir, "branch", "--show-current")
	status := gitCommand(dir, "status", "--short")

	if branch == "" && status == "" {
		return ""
	}

	var parts []string
	if branch != "" {
		parts = append(parts, "Branch: "+branch)
	}
	if status != "" {
		// Limit to first 20 lines
		lines := strings.Split(status, "\n")
		if len(lines) > 20 {
			lines = append(lines[:20], fmt.Sprintf("... and %d more files", len(lines)-20))
		}
		parts = append(parts, "Status:\n"+strings.Join(lines, "\n"))
	}
	return strings.Join(parts, "\n")
}

func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func gitCommand(dir string, args ...string) string {
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", fullArgs...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectShell() string {
	return shellName(os.Getenv("SHELL"))
}

// shellName returns the base shell name from a shell path (e.g. "/bin/zsh" ->
// "zsh"), or "unknown" when the path is empty.
func shellName(shellPath string) string {
	if shellPath == "" {
		return "unknown"
	}
	return filepath.Base(shellPath)
}
