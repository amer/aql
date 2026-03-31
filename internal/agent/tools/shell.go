package tools

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
