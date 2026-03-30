package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// AgentOutputMsg is sent when an agent produces new output.
type AgentOutputMsg struct {
	AgentName string
	Output    string
}

// AgentStatusMsg is sent when an agent's status changes.
type AgentStatusMsg struct {
	AgentName string
	Status    AgentStatus
	StatusMsg string
}

// AgentToolCallMsg is sent when an agent invokes a tool.
type AgentToolCallMsg struct {
	AgentName string
	ToolCall  ToolCall
}

// Model is the main Bubble Tea model for AQL.
type Model struct {
	workflowName string
	agents       []AgentPanelData
	agentIndex   map[string]int
	input        string
	width        int
	height       int
	submitted    []string
}

// NewModel creates the initial TUI model.
func NewModel(workflowName string, agentNames []string) Model {
	agents := make([]AgentPanelData, len(agentNames))
	index := make(map[string]int, len(agentNames))

	for i, name := range agentNames {
		agents[i] = AgentPanelData{
			Name:   name,
			Status: AgentWaiting,
		}
		index[name] = i
	}

	return Model{
		workflowName: workflowName,
		agents:       agents,
		agentIndex:   index,
		width:        80,
		height:       24,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if m.input != "" {
				m.submitted = append(m.submitted, m.input)
				m.input = ""
			}
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			if len(msg.String()) == 1 {
				m.input += msg.String()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case AgentOutputMsg:
		if idx, ok := m.agentIndex[msg.AgentName]; ok {
			m.agents[idx].Output = msg.Output
		}

	case AgentStatusMsg:
		if idx, ok := m.agentIndex[msg.AgentName]; ok {
			m.agents[idx].Status = msg.Status
			m.agents[idx].StatusMsg = msg.StatusMsg
		}

	case AgentToolCallMsg:
		if idx, ok := m.agentIndex[msg.AgentName]; ok {
			m.agents[idx].ToolCalls = append(m.agents[idx].ToolCalls, msg.ToolCall)
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	var b strings.Builder

	// Status bar
	b.WriteString(RenderStatusBar(m.workflowName, len(m.agents), m.width))
	b.WriteString("\n\n")

	// Agent panels
	for _, agent := range m.agents {
		b.WriteString(RenderAgentPanel(agent))
		b.WriteString("\n")
	}

	// Prompt at bottom
	b.WriteString(RenderPrompt(m.input, m.width))

	return b.String()
}

// Submitted returns all submitted inputs (for testing).
func (m Model) Submitted() []string {
	return m.submitted
}

// Input returns the current input text (for testing).
func (m Model) Input() string {
	return m.input
}

// AgentPanels returns the agent panel data (for testing).
func (m Model) AgentPanels() []AgentPanelData {
	return m.agents
}
