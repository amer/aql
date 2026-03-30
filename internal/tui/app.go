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

// AgentStreamDeltaMsg is sent for each streamed text chunk.
type AgentStreamDeltaMsg struct {
	AgentName string
	Delta     string
}

// AgentStreamDoneMsg is sent when streaming is complete.
type AgentStreamDoneMsg struct {
	AgentName string
}

// AgentStreamErrorMsg is sent when streaming encounters an error.
type AgentStreamErrorMsg struct {
	AgentName string
	Error     error
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

// SubmitFunc is called when the user submits input. It should return a tea.Cmd
// that kicks off agent processing.
type SubmitFunc func(input string) tea.Cmd

// Model is the main Bubble Tea model for AQL.
type Model struct {
	workflowName    string
	agentNames      []string
	chat            []ChatEntry
	input           string
	width           int
	height          int
	scrollOffset    int
	onSubmit        SubmitFunc
	streaming       bool
	spinnerFrame    int
	tokenCount      int
	modelName       string
	projectPath     string
	paletteVisible  bool
	paletteSelected int
	paletteFiltered []Command
}

// NewModel creates the initial TUI model.
func NewModel(workflowName string, agentNames []string, onSubmit SubmitFunc) Model {
	return Model{
		workflowName: workflowName,
		agentNames:   agentNames,
		width:        80,
		height:       24,
		onSubmit:     onSubmit,
		modelName:    "claude-sonnet-4",
		projectPath:  ".",
	}
}

// SetModelName sets the model name shown in the header and status bar.
func (m *Model) SetModelName(name string) {
	m.modelName = name
}

// SetProjectPath sets the project path shown in the header.
func (m *Model) SetProjectPath(path string) {
	m.projectPath = path
}

// SetTokenCount sets the token count shown in the status bar.
func (m *Model) SetTokenCount(count int) {
	m.tokenCount = count
}

// TokenCountMsg updates the token count in the status bar.
type TokenCountMsg struct {
	Count int
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
		case "esc":
			if m.paletteVisible {
				m.paletteVisible = false
				m.paletteSelected = 0
				m.input = ""
			}
		case "tab":
			if m.paletteVisible && len(m.paletteFiltered) > 0 {
				m.input = m.paletteFiltered[m.paletteSelected].Name
				m.paletteVisible = false
				m.paletteSelected = 0
			}
		case "alt+enter":
			m.input += "\n"
		case "enter":
			if m.input != "" && !m.streaming {
				cmd := strings.TrimSpace(m.input)
				m.paletteVisible = false
				m.paletteSelected = 0

				if cmd == "/exit" || cmd == "/quit" || cmd == "/q" {
					return m, tea.Quit
				}

				if result := m.executeCommand(cmd); result != "" {
					return m, nil
				}

				m.chat = append(m.chat, ChatEntry{
					Type:    EntryUserInput,
					Content: m.input,
				})
				userInput := m.input
				m.input = ""
				m.scrollToBottom()

				if m.onSubmit != nil {
					return m, m.onSubmit(userInput)
				}
			}
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
			m.updatePalette()
		case "up":
			if m.paletteVisible {
				if m.paletteSelected > 0 {
					m.paletteSelected--
				}
			} else if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		case "down":
			if m.paletteVisible {
				if m.paletteSelected < len(m.paletteFiltered)-1 {
					m.paletteSelected++
				}
			} else {
				m.scrollOffset++
			}
		default:
			if len(msg.String()) == 1 {
				m.input += msg.String()
			}
			m.updatePalette()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case SpinnerTickMsg:
		if m.streaming {
			m.spinnerFrame++
			return m, SpinnerTick()
		}

	case TokenCountMsg:
		m.tokenCount = msg.Count

	case AgentStreamDeltaMsg:
		wasStreaming := m.streaming
		m.streaming = true
		// Append to existing agent text entry or create new one
		if len(m.chat) > 0 {
			last := &m.chat[len(m.chat)-1]
			if last.Type == EntryAgentText && last.AgentName == msg.AgentName {
				last.Content += msg.Delta
				m.scrollToBottom()
				if !wasStreaming {
					return m, SpinnerTick()
				}
				return m, nil
			}
		}
		m.chat = append(m.chat, ChatEntry{
			Type:      EntryAgentText,
			AgentName: msg.AgentName,
			Content:   msg.Delta,
		})
		m.scrollToBottom()
		if !wasStreaming {
			return m, SpinnerTick()
		}

	case AgentStreamDoneMsg:
		m.streaming = false

	case AgentStreamErrorMsg:
		m.streaming = false
		m.chat = append(m.chat, ChatEntry{
			Type:      EntryAgentStatus,
			AgentName: msg.AgentName,
			Status:    AgentError,
			Content:   msg.Error.Error(),
		})
		m.scrollToBottom()

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

func (m *Model) updatePalette() {
	if strings.HasPrefix(m.input, "/") {
		m.paletteVisible = true
		m.paletteFiltered = FilterCommands(SlashCommands(), m.input)
		if m.paletteSelected >= len(m.paletteFiltered) {
			m.paletteSelected = 0
		}
	} else {
		m.paletteVisible = false
		m.paletteSelected = 0
	}
}

// executeCommand handles built-in slash commands. Returns non-empty if handled.
func (m *Model) executeCommand(cmd string) string {
	switch cmd {
	case "/exit", "/quit", "/q":
		// Handled separately — this shouldn't be reached since we check earlier,
		// but keep for safety.
		return ""
	case "/clear":
		m.chat = nil
		m.input = ""
		m.scrollToBottom()
		return "cleared"
	case "/help":
		var lines []string
		lines = append(lines, "Available commands:")
		for _, c := range SlashCommands() {
			lines = append(lines, "  "+c.Name+" — "+c.Description)
		}
		m.chat = append(m.chat, ChatEntry{
			Type:    EntryAgentStatus,
			Content: strings.Join(lines, "\n"),
			Status:  AgentActive,
		})
		m.input = ""
		m.scrollToBottom()
		return "help"
	case "/agents":
		names := strings.Join(m.agentNames, ", ")
		m.chat = append(m.chat, ChatEntry{
			Type:    EntryAgentStatus,
			Content: "Active agents: " + names,
			Status:  AgentActive,
		})
		m.input = ""
		m.scrollToBottom()
		return "agents"
	case "/status":
		status := "Workflow: " + m.workflowName + " · Agents: " + strings.Join(m.agentNames, ", ")
		m.chat = append(m.chat, ChatEntry{
			Type:    EntryAgentStatus,
			Content: status,
			Status:  AgentActive,
		})
		m.input = ""
		m.scrollToBottom()
		return "status"
	case "/model":
		m.chat = append(m.chat, ChatEntry{
			Type:    EntryAgentStatus,
			Content: "Model: " + m.modelName,
			Status:  AgentActive,
		})
		m.input = ""
		m.scrollToBottom()
		return "model"
	case "/compact":
		m.chat = append(m.chat, ChatEntry{
			Type:    EntryAgentStatus,
			Content: "Context compaction is not yet implemented",
			Status:  AgentWaiting,
		})
		m.input = ""
		m.scrollToBottom()
		return "compact"
	}
	return ""
}

// View implements tea.Model.
func (m Model) View() string {
	var b strings.Builder

	// Header
	header := RenderHeader(m.projectPath, m.modelName, m.width)
	b.WriteString(header)
	b.WriteString("\n\n")

	// Render all chat entries
	var chatLines []string
	for _, entry := range m.chat {
		chatLines = append(chatLines, RenderChatEntry(entry, m.width))
	}

	// Calculate visible area (header ~4 lines, prompt ~2 lines, status bar ~1 line)
	headerLines := strings.Count(header, "\n") + 1
	reservedLines := headerLines + 4 // prompt + status bar + padding
	visibleHeight := m.height - reservedLines

	chatContent := strings.Join(chatLines, "\n")
	contentLines := strings.Split(chatContent, "\n")

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

	// Prompt
	b.WriteString("\n")
	if m.streaming {
		b.WriteString(RenderPromptStreaming(m.spinnerFrame, m.agentName(), m.width))
	} else {
		b.WriteString(RenderPrompt(m.input, m.width))
	}

	// Command palette (below prompt, like Claude Code)
	if m.paletteVisible && len(m.paletteFiltered) > 0 {
		b.WriteString("\n")
		b.WriteString(RenderCommandPalette(m.paletteFiltered, m.paletteSelected, m.width))
	}

	// Status bar
	b.WriteString("\n")
	b.WriteString(RenderStatusBar(m.modelName, m.tokenCount, m.width))

	return b.String()
}

// agentName returns the first agent name for display, or "agent".
func (m Model) agentName() string {
	if len(m.agentNames) > 0 {
		return m.agentNames[0]
	}
	return "agent"
}

// RenderChatEntry renders a single chat entry.
func RenderChatEntry(entry ChatEntry, width int) string {
	switch entry.Type {
	case EntryUserInput:
		return RenderUserMessage(entry.Content)

	case EntryAgentText:
		header := RenderAgentHeader(entry.AgentName, AgentActive)
		rendered := RenderMarkdown(entry.Content, width-2)
		if rendered == "" {
			rendered = AgentBody.Render(entry.Content)
		}
		return header + "\n" + rendered

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

// IsStreaming returns whether the model is currently streaming (for testing).
func (m Model) IsStreaming() bool {
	return m.streaming
}

// IsPaletteVisible returns whether the command palette is visible (for testing).
func (m Model) IsPaletteVisible() bool {
	return m.paletteVisible
}

// PaletteSelected returns the currently selected palette index (for testing).
func (m Model) PaletteSelected() int {
	return m.paletteSelected
}

// PaletteCommands returns the currently filtered palette commands (for testing).
func (m Model) PaletteCommands() []Command {
	return m.paletteFiltered
}
