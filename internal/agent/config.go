package agent

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config defines the YAML configuration for an agent.
type Config struct {
	Name         string       `yaml:"name"`
	Role         string       `yaml:"role"`
	SystemPrompt string       `yaml:"system_prompt"`
	Model        string       `yaml:"model"`
	Tools        []string     `yaml:"tools"`
	Memory       MemoryConfig `yaml:"memory"`
	Events       EventsConfig `yaml:"events"`
}

// MemoryConfig defines memory access for an agent.
type MemoryConfig struct {
	Private      bool     `yaml:"private"`
	SharedAccess []string `yaml:"shared_access"`
}

// EventsConfig defines event pub/sub for an agent.
type EventsConfig struct {
	Publishes  []string `yaml:"publishes"`
	Subscribes []string `yaml:"subscribes"`
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
