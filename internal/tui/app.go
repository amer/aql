package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ChatEntry represents a single item in the scrolling chat log.
type ChatEntry struct {
	Type      ChatEntryType
	AgentName string
	Content   string
	ToolCall  *ToolCall
	Status    AgentStatus
}

// ChatEntryType identifies the kind of chat entry.
type ChatEntryType int

const (
	EntryUserInput ChatEntryType = iota
	EntryAgentText
	EntryAgentTool
	EntryAgentStatus
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
	agentNames   []string
	chat         []ChatEntry
	input        string
	width        int
	height       int
	scrollOffset int
}

// NewModel creates the initial TUI model.
func NewModel(workflowName string, agentNames []string) Model {
	return Model{
		workflowName: workflowName,
		agentNames:   agentNames,
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
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.input != "" {
				cmd := strings.TrimSpace(m.input)
				if cmd == "/exit" || cmd == "/quit" || cmd == "/q" {
					return m, tea.Quit
				}
				m.chat = append(m.chat, ChatEntry{
					Type:    EntryUserInput,
					Content: m.input,
				})
				m.input = ""
				m.scrollToBottom()
			}
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		case "up":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		case "down":
			m.scrollOffset++
		default:
			if len(msg.String()) == 1 {
				m.input += msg.String()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case AgentOutputMsg:
		m.chat = append(m.chat, ChatEntry{
			Type:      EntryAgentText,
			AgentName: msg.AgentName,
			Content:   msg.Output,
		})
		m.scrollToBottom()

	case AgentStatusMsg:
		m.chat = append(m.chat, ChatEntry{
			Type:      EntryAgentStatus,
			AgentName: msg.AgentName,
			Content:   msg.StatusMsg,
			Status:    msg.Status,
		})
		m.scrollToBottom()

	case AgentToolCallMsg:
		m.chat = append(m.chat, ChatEntry{
			Type:      EntryAgentTool,
			AgentName: msg.AgentName,
			ToolCall:  &msg.ToolCall,
		})
		m.scrollToBottom()
	}

	return m, nil
}

func (m *Model) scrollToBottom() {
	m.scrollOffset = len(m.chat)
}

// View implements tea.Model.
func (m Model) View() string {
	var b strings.Builder

	// Render all chat entries
	var chatLines []string
	for _, entry := range m.chat {
		chatLines = append(chatLines, RenderChatEntry(entry, m.width))
	}

	// Calculate visible area (leave room for prompt)
	chatContent := strings.Join(chatLines, "\n")
	contentLines := strings.Split(chatContent, "\n")
	visibleHeight := m.height - 3 // room for prompt

	// Show bottom of chat (auto-scroll)
	start := 0
	if len(contentLines) > visibleHeight {
		start = len(contentLines) - visibleHeight
	}
	if start < 0 {
		start = 0
	}

	visible := contentLines[start:]
	b.WriteString(strings.Join(visible, "\n"))

	// Pad to push prompt to bottom
	padding := visibleHeight - len(visible)
	for i := 0; i < padding; i++ {
		b.WriteString("\n")
	}

	// Prompt at bottom
	b.WriteString("\n")
	b.WriteString(RenderPrompt(m.input, m.width))

	return b.String()
}

// RenderChatEntry renders a single chat entry.
func RenderChatEntry(entry ChatEntry, width int) string {
	switch entry.Type {
	case EntryUserInput:
		return UserInputStyle.Render("> " + entry.Content)

	case EntryAgentText:
		header := RenderAgentHeader(entry.AgentName, AgentActive)
		body := AgentBody.Render(entry.Content)
		return header + "\n" + body

	case EntryAgentTool:
		if entry.ToolCall != nil {
			return RenderToolBlock(*entry.ToolCall)
		}
		return ""

	case EntryAgentStatus:
		header := RenderAgentHeader(entry.AgentName, entry.Status)
		if entry.Content != "" {
			return header + " " + DimStyle.Render(entry.Content)
		}
		return header

	default:
		return ""
	}
}

// Chat returns all chat entries (for testing).
func (m Model) Chat() []ChatEntry {
	return m.chat
}

// Input returns the current input text (for testing).
func (m Model) Input() string {
	return m.input
}
