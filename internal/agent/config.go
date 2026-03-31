package agent

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
