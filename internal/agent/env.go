package agent

import (
	"fmt"
	"strings"
)

// CheckEnv validates that the API key is set and non-empty.
func CheckEnv(apiKey string) error {
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY is not set\n\n  export ANTHROPIC_API_KEY=<your-key>")
	}
	return nil
}
