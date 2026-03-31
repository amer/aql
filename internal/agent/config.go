package agent

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Config struct definition, LoadConfig() and ParseConfig() for
//     YAML files.
//
// MUST NOT GO HERE:
//   - Agent construction logic (agent.go)
//   - Runtime config changes
//   - Environment variable handling (env.go)
//
// Q: Should I add a new config field?
// A: Add it to the Config struct here. Use it in agent.go's New() or
//    BuildPromptParts().
// ──────────────────────────────────────────────────────────────────

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config defines the configuration for an agent.
type Config struct {
	Name         string `yaml:"name"`
	Role         string `yaml:"role"`
	SystemPrompt string `yaml:"system_prompt"`
	Model        string `yaml:"model"`
}

// LoadConfig reads and parses an agent config from a YAML file.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	return ParseConfig(data)
}

// ParseConfig parses agent config from YAML bytes.
func ParseConfig(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}
