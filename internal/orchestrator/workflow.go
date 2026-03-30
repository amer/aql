package orchestrator

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Workflow defines how agents work together.
type Workflow struct {
	Name      string    `yaml:"name"`
	Agents    []string  `yaml:"agents"`
	Execution Execution `yaml:"execution"`
}

// Execution defines the execution strategy for a workflow.
type Execution struct {
	Mode  string `yaml:"mode"`
	Pairs []Pair `yaml:"pairs"`
}

// Pair defines a relationship between agents.
type Pair struct {
	Agents       []string `yaml:"agents"`
	Relationship string   `yaml:"relationship"`
}

// LoadWorkflow reads and parses a workflow from a YAML file.
func LoadWorkflow(path string) (Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Workflow{}, fmt.Errorf("read workflow: %w", err)
	}
	return ParseWorkflow(data)
}

// ParseWorkflow parses a workflow from YAML bytes.
func ParseWorkflow(data []byte) (Workflow, error) {
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return Workflow{}, fmt.Errorf("parse workflow: %w", err)
	}
	return wf, nil
}

// AllAgentsInPairs returns a deduplicated list of all agent names
// referenced in execution pairs.
func (w Workflow) AllAgentsInPairs() []string {
	seen := make(map[string]bool)
	var result []string
	for _, p := range w.Execution.Pairs {
		for _, a := range p.Agents {
			if !seen[a] {
				seen[a] = true
				result = append(result, a)
			}
		}
	}
	return result
}

// PairsForAgent returns all pairs that include the given agent.
func (w Workflow) PairsForAgent(agentName string) []Pair {
	var result []Pair
	for _, p := range w.Execution.Pairs {
		for _, a := range p.Agents {
			if a == agentName {
				result = append(result, p)
				break
			}
		}
	}
	return result
}
