package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShellName(t *testing.T) {
	assert.Equal(t, "zsh", shellName("/bin/zsh"))
	assert.Equal(t, "bash", shellName("/usr/local/bin/bash"))
	assert.Equal(t, "fish", shellName("fish"))
	assert.Equal(t, "unknown", shellName(""))
}
