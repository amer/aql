package tui

import (
	"fmt"
	"strings"
	"time"

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

// AgentStreamStartMsg is sent when the agent starts processing (before API call).
type AgentStreamStartMsg struct {
	AgentName string
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

// ModelsLoadedMsg is sent when background model probing completes.
type ModelsLoadedMsg struct {
	Tiers []ModelTier
}

// BashResultMsg is sent when a ! bash command completes.
type BashResultMsg struct {
	Command string
	Output  string
	Error   error
}

// BashFunc executes a shell command and returns a tea.Cmd with the result.
// Set by the main app to provide actual shell execution.
type BashFunc func(command string) tea.Cmd

// SubmitFunc is called when the user submits input. It should return a tea.Cmd
// that kicks off agent processing.
type SubmitFunc func(input string) tea.Cmd

// Model is the main Bubble Tea model for AQL.
type Model struct {
	workflowName       string
	agentNames         []string
	chat               []ChatEntry
	inputBuf           *InputBuffer
	history            *History
	width              int
	height             int
	scrollOffset       int
	onSubmit           SubmitFunc
	onBash             BashFunc
	streaming          bool
	spinnerFrame       int
	streamStart        time.Time // when current streaming started
	streamChars        int       // chars received in current stream
	tokenCount         int
	modelName          string
	projectPath        string
	paletteVisible     bool
	paletteSelected    int
	paletteFiltered    []Command
	onModelSelected    func(modelID string)
	modelPickerVisible bool
	modelPickerIdx     int
	modelPickerInput   string
	spinnerType        SpinnerType
	modelTiers         []ModelTier // dynamic model tiers from API; nil = use defaults
	cancelStream       func()      // cancels the in-flight API call context
}

// NewModel creates the initial TUI model.
func NewModel(workflowName string, agentNames []string, onSubmit SubmitFunc) Model {
	return Model{
		workflowName: workflowName,
		agentNames:   agentNames,
		inputBuf:     NewInputBuffer(),
		history:      NewHistory(),
		width:        80,
		height:       24,
		onSubmit:     onSubmit,
		modelName:    "claude-sonnet-4-6",
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

// SetOnBash sets the callback for executing ! bash commands.
func (m *Model) SetOnBash(fn BashFunc) {
	m.onBash = fn
}

// SetCancelStream sets the function called to cancel an in-flight API call
// when the user exits during streaming (Ctrl+C / Ctrl+D).
func (m *Model) SetCancelStream(fn func()) {
	m.cancelStream = fn
}

// SetOnModelSelected sets a callback invoked when the user switches models.
func (m *Model) SetOnModelSelected(fn func(modelID string)) {
	m.onModelSelected = fn
}

// SetTokenCount sets the token count shown in the status bar.
func (m *Model) SetTokenCount(count int) {
	m.tokenCount = count
}

// SetModelTiers sets the dynamic model tiers for the model picker.
func (m *Model) SetModelTiers(tiers []ModelTier) {
	m.modelTiers = tiers
}

// GetModelTiers returns the current model tiers, falling back to defaults.
func (m Model) GetModelTiers() []ModelTier {
	if len(m.modelTiers) > 0 {
		return m.modelTiers
	}
	return DefaultModelTiers()
}

// SetBootstrapping is a no-op kept for API compatibility.
// Model probing runs silently in the background.
func (m *Model) SetBootstrapping(_ bool) {}

// IsBootstrapping always returns false — bootstrapping is invisible.
func (m Model) IsBootstrapping() bool { return false }

// SetSpinnerType sets the active spinner animation style.
func (m *Model) SetSpinnerType(st SpinnerType) {
	m.spinnerType = st
}

// ActiveSpinnerType returns the current spinner animation style.
func (m Model) ActiveSpinnerType() SpinnerType {
	return m.spinnerType
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
		if m.modelPickerVisible {
			return m.handleModelPickerKey(msg)
		}
		return m.handleKey(msg)

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scrollUp(5)
		case tea.MouseButtonWheelDown:
			m.scrollDown(5)
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	default:
		return m.handleMsg(msg)
	}

	return m, nil
}

func (m Model) handleModelPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	tiers := m.GetModelTiers()
	maxIdx := len(tiers) // includes "Use custom ID" entry
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.modelPickerVisible = false
		m.modelPickerIdx = 0
		m.modelPickerInput = ""
	case "up":
		if m.modelPickerIdx > 0 {
			m.modelPickerIdx--
		}
	case "down":
		if m.modelPickerIdx < maxIdx {
			m.modelPickerIdx++
		}
	case "backspace":
		if len(m.modelPickerInput) > 0 {
			m.modelPickerInput = m.modelPickerInput[:len(m.modelPickerInput)-1]
		}
	case "enter":
		if m.modelPickerIdx < len(tiers) {
			tier := tiers[m.modelPickerIdx]
			return m, m.selectModel(tier.ModelID, tier.Label+" ("+tier.ModelID+")")
		}
		if m.modelPickerInput != "" {
			return m, m.selectModel(m.modelPickerInput, m.modelPickerInput)
		}
	default:
		if m.modelPickerIdx == len(tiers) && len(msg.String()) == 1 {
			m.modelPickerInput += msg.String()
		}
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "ctrl+d":
		if m.streaming && m.cancelStream != nil {
			m.cancelStream()
		}
		return m, tea.Quit
	case "esc":
		if m.paletteVisible {
			m.paletteVisible = false
			m.paletteSelected = 0
			m.inputBuf.Clear()
		}
	case "tab":
		if m.paletteVisible && len(m.paletteFiltered) > 0 {
			m.inputBuf.Set(m.paletteFiltered[m.paletteSelected].Name)
			m.paletteVisible = false
			m.paletteSelected = 0
		}
	case "alt+enter", "ctrl+j":
		m.inputBuf.Insert('\n')
	case "enter":
		return m.handleSubmit()
	case "backspace":
		m.inputBuf.DeleteBackward()
		m.updatePalette()
	case "left":
		m.inputBuf.MoveLeft()
	case "right":
		m.inputBuf.MoveRight()
	case "ctrl+a", "home":
		m.inputBuf.MoveToStart()
	case "ctrl+e", "end":
		m.inputBuf.MoveToEnd()
	case "ctrl+k":
		m.inputBuf.KillToEnd()
		m.updatePalette()
	case "ctrl+u":
		m.inputBuf.KillToStart()
		m.updatePalette()
	case "alt+p":
		m.openModelPicker()
	case "shift+up":
		m.scrollUp(3)
	case "shift+down":
		m.scrollDown(3)
	case "pgup":
		m.scrollUp(m.visibleHeight() / 2)
	case "pgdown":
		m.scrollDown(m.visibleHeight() / 2)
	case "up":
		if m.paletteVisible {
			if m.paletteSelected > 0 {
				m.paletteSelected--
			}
		} else if val, ok := m.history.Previous(); ok {
			m.inputBuf.Set(val)
		}
	case "down":
		if m.paletteVisible {
			if m.paletteSelected < len(m.paletteFiltered)-1 {
				m.paletteSelected++
			}
		} else if val, ok := m.history.Next(); ok {
			m.inputBuf.Set(val)
		}
	default:
		if msg.Paste {
			m.inputBuf.InsertString(string(msg.Runes))
		} else if len(msg.String()) == 1 {
			m.inputBuf.Insert(rune(msg.String()[0]))
		}
		m.updatePalette()
	}
	return m, nil
}

func (m Model) handleSubmit() (tea.Model, tea.Cmd) {
	input := m.inputBuf.String()
	if input == "" || m.streaming {
		return m, nil
	}

	// Resolve command: palette selection takes priority
	cmd := strings.TrimSpace(input)
	if m.paletteVisible && len(m.paletteFiltered) > 0 {
		cmd = m.paletteFiltered[m.paletteSelected].Name
	}
	m.paletteVisible = false
	m.paletteSelected = 0

	if cmd == "/exit" || cmd == "/quit" || cmd == "/q" {
		return m, tea.Quit
	}
	if result, resultCmd := m.executeCommand(cmd); result != "" {
		return m, resultCmd
	}

	// ! bash mode
	if IsBashCommand(input) {
		shellCmd := ParseBashCommand(input)
		if shellCmd != "" {
			m.history.Push(input)
			m.chat = append(m.chat, ChatEntry{Type: EntryUserInput, Content: input})
			m.inputBuf.Clear()
			m.scrollToBottom()
			if m.onBash != nil {
				return m, m.onBash(shellCmd)
			}
		}
		return m, nil
	}

	// Normal submit
	m.history.Push(input)
	m.chat = append(m.chat, ChatEntry{Type: EntryUserInput, Content: input})
	m.inputBuf.Clear()
	m.scrollToBottom()
	if m.onSubmit != nil {
		return m, m.onSubmit(input)
	}
	return m, nil
}

func (m *Model) openModelPicker() {
	m.modelPickerVisible = true
	m.modelPickerIdx = 0
	m.modelPickerInput = ""
	for i, tier := range m.GetModelTiers() {
		if tier.ModelID == m.modelName {
			m.modelPickerIdx = i
			break
		}
	}
}

func (m Model) handleMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SpinnerTickMsg:
		if m.streaming {
			m.spinnerFrame++
			return m, SpinnerTickFor(m.spinnerType)
		}

	case TokenCountMsg:
		m.tokenCount = msg.Count

	case AgentStreamStartMsg:
		if !m.streaming {
			m.startStream()
			return m, SpinnerTickFor(m.spinnerType)
		}

	case AgentStreamDeltaMsg:
		wasStreaming := m.streaming
		if !wasStreaming {
			m.startStream()
		}
		m.streamChars += len(msg.Delta)
		// Append to existing agent text entry or create new one
		if len(m.chat) > 0 {
			last := &m.chat[len(m.chat)-1]
			if last.Type == EntryAgentText && last.AgentName == msg.AgentName {
				last.Content += msg.Delta
				m.autoScroll()
				if !wasStreaming {
					return m, SpinnerTickFor(m.spinnerType)
				}
				return m, nil
			}
		}
		m.chat = append(m.chat, ChatEntry{
			Type:      EntryAgentText,
			AgentName: msg.AgentName,
			Content:   msg.Delta,
		})
		m.autoScroll()
		if !wasStreaming {
			return m, SpinnerTickFor(m.spinnerType)
		}

	case AgentStreamDoneMsg:
		m.tokenCount += EstimateTokens(m.streamChars)
		elapsed := time.Since(m.streamStart)
		m.streaming = false
		m.chat = append(m.chat, ChatEntry{
			Type:    EntryAgentStatus,
			Content: "\n" + RenderCompletionIndicator(elapsed) + "\n",
			Status:  AgentDone,
		})
		m.autoScroll()

	case AgentStreamErrorMsg:
		m.streaming = false
		m.chat = append(m.chat, ChatEntry{
			Type:      EntryAgentStatus,
			AgentName: msg.AgentName,
			Status:    AgentError,
			Content:   msg.Error.Error(),
		})
		m.autoScroll()

	case AgentOutputMsg:
		m.chat = append(m.chat, ChatEntry{
			Type:      EntryAgentText,
			AgentName: msg.AgentName,
			Content:   msg.Output,
		})
		m.autoScroll()

	case AgentStatusMsg:
		m.chat = append(m.chat, ChatEntry{
			Type:      EntryAgentStatus,
			AgentName: msg.AgentName,
			Content:   msg.StatusMsg,
			Status:    msg.Status,
		})
		m.autoScroll()

	case BashResultMsg:
		content := msg.Output
		status := AgentActive
		if msg.Error != nil {
			content += "\n" + msg.Error.Error()
			status = AgentError
		}
		m.chat = append(m.chat, ChatEntry{
			Type:    EntryAgentStatus,
			Content: "$ " + msg.Command + "\n" + content,
			Status:  status,
		})
		m.autoScroll()

	case ModelsLoadedMsg:
		if len(msg.Tiers) > 0 {
			m.modelTiers = msg.Tiers
		}

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
		m.autoScroll()
	}

	return m, nil
}

func (m *Model) startStream() {
	m.spinnerType = RandomSpinnerType()
	m.spinnerFrame = 0
	m.streamStart = time.Now()
	m.streamChars = 0
	m.streaming = true
}

// selectModel handles model selection from the picker or custom input.
func (m *Model) selectModel(modelID string, label string) tea.Cmd {
	m.modelName = modelID
	m.modelPickerVisible = false
	m.modelPickerIdx = 0
	m.modelPickerInput = ""
	m.chat = append(m.chat, ChatEntry{
		Type:    EntryAgentStatus,
		Content: "Switched to: " + label,
		Status:  AgentActive,
	})
	m.scrollToBottom()
	return func() tea.Msg {
		return ModelSelectedMsg{Model: modelID}
	}
}

func (m *Model) scrollToBottom() {
	m.scrollOffset = 0
}

// scrollUp increases scroll offset (moves view up) by delta lines.
func (m *Model) scrollUp(delta int) {
	m.scrollOffset += delta
	// Clamping happens at render time since we don't track total lines here.
}

// scrollDown decreases scroll offset (moves view down) by delta lines.
func (m *Model) scrollDown(delta int) {
	m.scrollOffset -= delta
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// visibleHeight returns the number of content lines visible in the chat area.
func (m Model) visibleHeight() int {
	header := RenderHeader(m.projectPath, m.modelName, m.width)
	headerLines := strings.Count(header, "\n") + 1
	reservedLines := headerLines + 4 // prompt + status bar + padding
	h := m.height - reservedLines
	if h < 1 {
		h = 1
	}
	return h
}

// isAtBottom returns true when the view is scrolled to the bottom.
func (m Model) isAtBottom() bool {
	return m.scrollOffset == 0
}

// autoScroll scrolls to bottom only if the user hasn't scrolled up.
func (m *Model) autoScroll() {
	if m.isAtBottom() {
		m.scrollToBottom()
	}
}

func (m *Model) updatePalette() {
	input := m.inputBuf.String()
	if strings.HasPrefix(input, "/") {
		m.paletteVisible = true
		m.paletteFiltered = FilterCommands(SlashCommands(), input)
		if m.paletteSelected >= len(m.paletteFiltered) {
			m.paletteSelected = 0
		}
	} else {
		m.paletteVisible = false
		m.paletteSelected = 0
	}
}

// addStatusChat appends a status message to chat, clears input, and scrolls to bottom.
func (m *Model) addStatusChat(content string, status AgentStatus) {
	m.chat = append(m.chat, ChatEntry{
		Type:    EntryAgentStatus,
		Content: content,
		Status:  status,
	})
	m.inputBuf.Clear()
	m.scrollToBottom()
}

// executeCommand handles built-in slash commands. Returns non-empty string if handled,
// and optionally a tea.Cmd to execute.
func (m *Model) executeCommand(cmd string) (string, tea.Cmd) {
	switch cmd {
	case "/exit", "/quit", "/q":
		return "", nil
	case "/clear":
		m.chat = nil
		m.inputBuf.Clear()
		m.scrollToBottom()
		return "cleared", nil
	case "/help":
		var lines []string
		lines = append(lines, "Available commands:")
		for _, c := range SlashCommands() {
			lines = append(lines, "  "+c.Name+" — "+c.Description)
		}
		m.addStatusChat(strings.Join(lines, "\n"), AgentActive)
		return "help", nil
	case "/agents":
		m.addStatusChat("Active agents: "+strings.Join(m.agentNames, ", "), AgentActive)
		return "agents", nil
	case "/status":
		m.addStatusChat("Workflow: "+m.workflowName+" · Agents: "+strings.Join(m.agentNames, ", "), AgentActive)
		return "status", nil
	case "/model":
		m.modelPickerVisible = true
		m.modelPickerIdx = 0
		m.modelPickerInput = ""
		for i, tier := range m.GetModelTiers() {
			if tier.ModelID == m.modelName {
				m.modelPickerIdx = i
				break
			}
		}
		m.inputBuf.Clear()
		return "model", nil
	case "/cost":
		usage := fmt.Sprintf("Token usage: %s (%s)", FormatTokenCount(m.tokenCount), FormatTokenCountShort(m.tokenCount))
		m.addStatusChat(usage, AgentActive)
		return "cost", nil
	case "/compact":
		m.addStatusChat("Context compaction is not yet implemented", AgentWaiting)
		return "compact", nil
	case "/spinner":
		types := SpinnerTypes()
		next := SpinnerBraille
		for i, st := range types {
			if st == m.spinnerType {
				next = types[(i+1)%len(types)]
				break
			}
		}
		m.spinnerType = next
		def := SpinnerDef(next)
		m.addStatusChat("Spinner: "+def.Name+" "+def.Frames[0], AgentActive)
		return "spinner", nil
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

	// Calculate visible area
	vh := m.visibleHeight()

	chatContent := strings.Join(chatLines, "\n")
	contentLines := strings.Split(chatContent, "\n")

	// Clamp scrollOffset to valid range
	maxOffset := len(contentLines) - vh
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := m.scrollOffset
	if offset > maxOffset {
		offset = maxOffset
	}

	// Slice the visible window: end is from-bottom, start is end-vh
	end := len(contentLines) - offset
	start := end - vh
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}

	visible := contentLines[start:end]
	b.WriteString(strings.Join(visible, "\n"))

	// Pad to push prompt to bottom
	padding := vh - len(visible)
	for i := 0; i < padding; i++ {
		b.WriteString("\n")
	}

	// Scroll indicator when not at bottom
	if offset > 0 {
		indicator := fmt.Sprintf(" ↓ %d more lines below ", offset)
		b.WriteString("\n")
		b.WriteString(DimStyle.Render(indicator))
	}

	// Streaming indicator (above prompt, like Claude Code)
	if m.streaming {
		status := StreamStatus{
			Elapsed: time.Since(m.streamStart).Truncate(time.Second),
			Tokens:  EstimateTokens(m.streamChars),
		}
		b.WriteString("\n")
		b.WriteString(RenderStreamingIndicator(m.spinnerFrame, m.agentName(), status, m.spinnerType))
	}

	// Prompt area with separator lines and project badge
	b.WriteString("\n")
	b.WriteString(RenderPromptArea(m.inputBuf.RenderWithCursor(), m.projectPath, m.width))

	// Command palette (below prompt, like Claude Code)
	if m.paletteVisible && len(m.paletteFiltered) > 0 {
		b.WriteString("\n")
		b.WriteString(RenderCommandPalette(m.paletteFiltered, m.paletteSelected, m.width))
	}

	// Model picker (below prompt)
	if m.modelPickerVisible {
		b.WriteString("\n")
		b.WriteString(RenderModelPicker(m.GetModelTiers(), m.modelPickerIdx, m.modelName, m.width))
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
	return m.inputBuf.String()
}

// ScrollOffset returns how many lines the view is scrolled up from the bottom (for testing).
// 0 means at the bottom.
func (m Model) ScrollOffset() int {
	return m.scrollOffset
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
