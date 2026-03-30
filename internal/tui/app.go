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

// ModelSelectedMsg is emitted when the user selects a model via /model <name>.
// The main app should persist this selection and reconfigure agents.
type ModelSelectedMsg struct {
	Model string
}

// SubmitFunc is called when the user submits input. It should return a tea.Cmd
// that kicks off agent processing.
type SubmitFunc func(input string) tea.Cmd

// ModelOption represents a selectable model in the TUI.
type ModelOption struct {
	ID          string // full model ID (e.g. "claude-sonnet-4-20250514")
	DisplayName string // human-readable name (e.g. "Claude Sonnet 4")
}

// Model is the main Bubble Tea model for AQL.
type Model struct {
	workflowName       string
	agentNames         []string
	chat               []ChatEntry
	input              string
	width              int
	height             int
	scrollOffset       int
	onSubmit           SubmitFunc
	streaming          bool
	spinnerFrame       int
	tokenCount         int
	modelName          string
	projectPath        string
	paletteVisible     bool
	paletteSelected    int
	paletteFiltered    []Command
	availableModels    []ModelOption
	onModelSelected    func(modelID string)
	modelPickerVisible bool
	modelPickerIdx     int
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

// SetAvailableModels sets the list of models shown by /model.
func (m *Model) SetAvailableModels(models []ModelOption) {
	m.availableModels = models
}

// SetOnModelSelected sets a callback invoked when the user switches models.
func (m *Model) SetOnModelSelected(fn func(modelID string)) {
	m.onModelSelected = fn
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
		// Model picker intercepts keys when visible
		if m.modelPickerVisible {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.modelPickerVisible = false
				m.modelPickerIdx = 0
				return m, nil
			case "up":
				if m.modelPickerIdx > 0 {
					m.modelPickerIdx--
				}
				return m, nil
			case "down":
				if m.modelPickerIdx < len(m.availableModels)-1 {
					m.modelPickerIdx++
				}
				return m, nil
			case "enter":
				if len(m.availableModels) > 0 {
					selected := m.availableModels[m.modelPickerIdx]
					m.modelName = selected.ID
					m.modelPickerVisible = false
					m.modelPickerIdx = 0
					m.chat = append(m.chat, ChatEntry{
						Type:    EntryAgentStatus,
						Content: "Switched to: " + selected.DisplayName + " (" + selected.ID + ")",
						Status:  AgentActive,
					})
					m.scrollToBottom()
					selectedID := selected.ID
					return m, func() tea.Msg {
						return ModelSelectedMsg{Model: selectedID}
					}
				}
				return m, nil
			}
			return m, nil
		}

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

				if result, resultCmd := m.executeCommand(cmd); result != "" {
					return m, resultCmd
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

	case ModelSelectedMsg:
		if m.onModelSelected != nil {
			m.onModelSelected(msg.Model)
		}

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

// executeCommand handles built-in slash commands. Returns non-empty string if handled,
// and optionally a tea.Cmd to execute.
func (m *Model) executeCommand(cmd string) (string, tea.Cmd) {
	// Handle /model <query> — match by ID substring or display name
	if strings.HasPrefix(cmd, "/model ") {
		query := strings.TrimSpace(strings.TrimPrefix(cmd, "/model"))
		queryLower := strings.ToLower(query)
		var match *ModelOption
		for i, opt := range m.availableModels {
			if strings.Contains(strings.ToLower(opt.ID), queryLower) ||
				strings.Contains(strings.ToLower(opt.DisplayName), queryLower) {
				match = &m.availableModels[i]
				break
			}
		}
		if match == nil {
			m.chat = append(m.chat, ChatEntry{
				Type:    EntryAgentStatus,
				Content: "No model matching: " + query + ". Use /model to see available options.",
				Status:  AgentError,
			})
			m.input = ""
			m.scrollToBottom()
			return "model", nil
		}
		m.modelName = match.ID
		m.chat = append(m.chat, ChatEntry{
			Type:    EntryAgentStatus,
			Content: "Switched to: " + match.DisplayName + " (" + match.ID + ")",
			Status:  AgentActive,
		})
		m.input = ""
		m.scrollToBottom()
		selectedID := match.ID
		return "model", func() tea.Msg {
			return ModelSelectedMsg{Model: selectedID}
		}
	}

	switch cmd {
	case "/exit", "/quit", "/q":
		return "", nil
	case "/clear":
		m.chat = nil
		m.input = ""
		m.scrollToBottom()
		return "cleared", nil
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
		return "help", nil
	case "/agents":
		names := strings.Join(m.agentNames, ", ")
		m.chat = append(m.chat, ChatEntry{
			Type:    EntryAgentStatus,
			Content: "Active agents: " + names,
			Status:  AgentActive,
		})
		m.input = ""
		m.scrollToBottom()
		return "agents", nil
	case "/status":
		status := "Workflow: " + m.workflowName + " · Agents: " + strings.Join(m.agentNames, ", ")
		m.chat = append(m.chat, ChatEntry{
			Type:    EntryAgentStatus,
			Content: status,
			Status:  AgentActive,
		})
		m.input = ""
		m.scrollToBottom()
		return "status", nil
	case "/model":
		// Open interactive model picker
		m.modelPickerVisible = true
		m.modelPickerIdx = 0
		// Pre-select current model
		for i, opt := range m.availableModels {
			if opt.ID == m.modelName {
				m.modelPickerIdx = i
				break
			}
		}
		m.input = ""
		return "model", nil
	case "/compact":
		m.chat = append(m.chat, ChatEntry{
			Type:    EntryAgentStatus,
			Content: "Context compaction is not yet implemented",
			Status:  AgentWaiting,
		})
		m.input = ""
		m.scrollToBottom()
		return "compact", nil
	}
	return "", nil
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

	// Prompt area with separator lines and project badge
	b.WriteString("\n")
	if m.streaming {
		b.WriteString(RenderPromptAreaStreaming(m.spinnerFrame, m.agentName(), m.projectPath, m.width))
	} else {
		b.WriteString(RenderPromptArea(m.input, m.projectPath, m.width))
	}

	// Command palette (below prompt, like Claude Code)
	if m.paletteVisible && len(m.paletteFiltered) > 0 {
		b.WriteString("\n")
		b.WriteString(RenderCommandPalette(m.paletteFiltered, m.paletteSelected, m.width))
	}

	// Model picker (below prompt)
	if m.modelPickerVisible && len(m.availableModels) > 0 {
		b.WriteString("\n")
		b.WriteString(RenderModelPicker(m.availableModels, m.modelPickerIdx, m.modelName, m.width))
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

// IsModelPickerVisible returns whether the model picker is visible (for testing).
func (m Model) IsModelPickerVisible() bool {
	return m.modelPickerVisible
}

// ModelPickerSelected returns the currently selected picker index (for testing).
func (m Model) ModelPickerSelected() int {
	return m.modelPickerIdx
}
