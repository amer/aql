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
