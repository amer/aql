package agent_test

import (
	"testing"

	"github.com/amer/aql/internal/agent"
	"github.com/stretchr/testify/assert"
)

func TestCheckEnvMissingKey(t *testing.T) {
	err := agent.CheckEnv("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY is not set")
}

func TestCheckEnvWithKey(t *testing.T) {
	err := agent.CheckEnv("sk-ant-test-key")
	assert.NoError(t, err)
}

func TestCheckEnvWhitespaceOnly(t *testing.T) {
	err := agent.CheckEnv("   ")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY is not set")
}

func TestEnvironmentInfo_ContainsDate(t *testing.T) {
	info := agent.EnvironmentInfo("/tmp/test", "claude-sonnet-4-6")
	assert.Contains(t, info, "Date:")
	// Should contain YYYY-MM-DD format
	assert.Regexp(t, `\d{4}-\d{2}-\d{2}`, info)
}

func TestEnvironmentInfo_ContainsPlatform(t *testing.T) {
	info := agent.EnvironmentInfo("/tmp/test", "claude-sonnet-4-6")
	assert.Contains(t, info, "Platform:")
}

func TestEnvironmentInfo_ContainsCWD(t *testing.T) {
	info := agent.EnvironmentInfo("/home/user/project", "claude-sonnet-4-6")
	assert.Contains(t, info, "/home/user/project")
}

func TestEnvironmentInfo_ContainsModel(t *testing.T) {
	info := agent.EnvironmentInfo("/tmp", "claude-opus-4-6")
	assert.Contains(t, info, "claude-opus-4-6")
}

func TestEnvironmentInfo_DetectsGitRepo(t *testing.T) {
	// Current test dir should be a git repo (we're in the aql project)
	info := agent.EnvironmentInfo(".", "claude-sonnet-4-6")
	assert.Contains(t, info, "Git repo:")
}

func TestGitStatus_ReturnsString(t *testing.T) {
	// Should not error even in a non-git directory
	status := agent.GitStatus("/tmp")
	assert.NotNil(t, status)
}
