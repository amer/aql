package agent

import (
	"fmt"
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
	// Check SHELL env var first
	for _, env := range []string{"SHELL"} {
		cmd := exec.Command("sh", "-c", "echo $"+env)
		out, err := cmd.Output()
		if err == nil {
			s := strings.TrimSpace(string(out))
			if s != "" {
				return filepath.Base(s)
			}
		}
	}
	return "unknown"
}
