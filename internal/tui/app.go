package tui

import (
	"fmt"
	"os/user"
	"strings"
	"time"

	"github.com/amer/aql/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

// Version is the application version, set at build time via ldflags.
var Version = "dev"

// ChatEntry represents a single item in the scrolling chat log.
type ChatEntry struct {
	Type      ChatEntryType
	AgentName string
	Content   string
	ToolCall  *domain.ToolCall
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
	ToolCall  domain.ToolCall
}

// AgentAskUserMsg is sent when the agent uses ask_user to request user input.
// The TUI shows the question and sends the user's answer back via ResponseCh.
type AgentAskUserMsg struct {
	AgentName  string
	Question   string
	ResponseCh chan<- string
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

// CompactDoneMsg is sent when context compaction completes.
type CompactDoneMsg struct {
	Summary string
	Err     error
}

// TokenUsageMsg is sent with precise token counts from the API response.
type TokenUsageMsg = domain.TokenUsageEvent

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
	streamStart        time.Time   // when current streaming started
	streamChars        int         // chars received in current stream
	streamPhase        StreamPhase // current streaming phase (requesting/responding)
	tokenCount         int
	modelName          string
	projectPath        string
	paletteVisible     bool
	paletteSelected    int
	paletteFiltered    []Command
	paletteMaxItems    int // high-water mark: max items shown during this palette session
	onModelSelected    func(modelID string)
	modelPickerVisible bool
	modelPickerIdx     int
	modelPickerInput   string
	spinnerType        SpinnerType
	modelTiers         []ModelTier      // dynamic model tiers from API; nil = use defaults
	cancelStream       func()           // cancels the in-flight API call context
	onClear            func()           // called on /clear to reset agent context
	onCompact          func() tea.Cmd   // called on /compact to summarize context
	selection          Selection        // mouse text selection state
	viewLines          []string         // plain text lines from last render (for selection extraction)
	pendingQuestion    *AgentAskUserMsg // non-nil when agent is waiting for user answer
	transcriptMode     bool             // Ctrl+O: expand all tools, enable search
	tsSearching        bool             // actively typing a search query in transcript mode
	tsSearchQuery      string           // current transcript search query
	tsMatches          []int            // block indices matching the search
	tsMatchIdx         int              // current match index
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

// welcomeData builds the WelcomeData struct from the current model state.
func (m Model) welcomeData() WelcomeData {
	username := ""
	homeDir := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
		homeDir = u.HomeDir
	}
	return WelcomeData{
		AppName:     "AQL",
		Version:     Version,
		ProjectPath: m.projectPath,
		HomeDir:     homeDir,
		ModelName:   m.modelName,
		Username:    username,
		Width:       m.width,
	}
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

// SetOnClear sets the function called when /clear is executed to reset agent context.
func (m *Model) SetOnClear(fn func()) {
	m.onClear = fn
}

// SetOnCompact sets the function called when /compact is executed to summarize context.
func (m *Model) SetOnCompact(fn func() tea.Cmd) {
	m.onCompact = fn
}

// SetOnModelSelected sets a callback invoked when the user switches models.
func (m *Model) SetOnModelSelected(fn func(modelID string)) {
	m.onModelSelected = fn
}

// TokenCount returns the current token count.
func (m Model) TokenCount() int {
	return m.tokenCount
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
		switch {
		case msg.Button == tea.MouseButtonWheelUp:
			m.scrollUp(5)
		case msg.Button == tea.MouseButtonWheelDown:
			m.scrollDown(5)
		case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
			m.computeViewLines() // snapshot screen content for selection extraction
			m.selection.Start(msg.X, msg.Y)
		case msg.Action == tea.MouseActionMotion && m.selection.Active():
			m.selection.Update(msg.X, msg.Y)
		case msg.Action == tea.MouseActionRelease && m.selection.Active():
			m.selection.Update(msg.X, msg.Y)
			text := m.selection.Extract(m.viewLines)
			m.selection.Clear()
			if text != "" {
				return m, copyToClipboard(text)
			}
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
	// Reserve lines for: prompt area (4: \n + top-bar + input + bottom-bar),
	// status bar (2: \n + content), scroll indicator (1: \n + text).
	reservedLines := 7
	if m.streaming {
		reservedLines++ // streaming indicator (\n + text)
	}
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

// computeViewLines builds the plain text lines currently visible on screen.
// Uses the actual View() output so coordinates match exactly what the terminal shows.
func (m *Model) computeViewLines() {
	rendered := m.View()
	lines := strings.Split(rendered, "\n")
	m.viewLines = make([]string, len(lines))
	for i, line := range lines {
		m.viewLines[i] = stripAnsiString(line)
	}
}

// applySelectionHighlight overlays reverse-video ANSI on selected characters.
func (m Model) applySelectionHighlight(view string) string {
	sx, sy, ex, ey := m.selection.Normalized()
	lines := strings.Split(view, "\n")

	for y := sy; y <= ey && y < len(lines); y++ {
		fromCol := 0
		toCol := -1 // -1 means to end of line
		if y == sy {
			fromCol = sx
		}
		if y == ey {
			toCol = ex
		}
		lines[y] = highlightLineRange(lines[y], fromCol, toCol)
	}

	return strings.Join(lines, "\n")
}

func (m *Model) updatePalette() {
	input := m.inputBuf.String()
	if strings.HasPrefix(input, "/") {
		m.paletteVisible = true
		m.paletteFiltered = FilterCommands(SlashCommands(), input)
		if m.paletteSelected >= len(m.paletteFiltered) {
			m.paletteSelected = 0
		}
		// Track high-water mark so the palette area never shrinks while typing
		if len(m.paletteFiltered) > m.paletteMaxItems {
			m.paletteMaxItems = len(m.paletteFiltered)
		}
	} else {
		m.paletteVisible = false
		m.paletteSelected = 0
		m.paletteMaxItems = 0
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
		// Reset the agent's API conversation history too — without this,
		// /clear would only hide messages visually while the agent still
		// carries the full prior context into the next API call.
		if m.onClear != nil {
			m.onClear()
		}
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
		if m.onCompact == nil {
			m.addStatusChat("Compact is not available", AgentError)
			return "compact", nil
		}
		m.chat = nil
		m.addStatusChat("Compacting conversation...", AgentWaiting)
		m.startStream()
		m.streamPhase = PhaseRequesting
		return "compact", m.onCompact()
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

	// Render chat as transcript blocks, with welcome banner at top
	var chatLines []string
	welcome := RenderWelcome(m.welcomeData())
	chatLines = append(chatLines, welcome)
	chatLines = append(chatLines, "") // blank separator
	blocks := BuildTranscriptBlocks(m.chat)
	for _, block := range blocks {
		chatLines = append(chatLines, RenderTranscriptBlock(block, m.width, m.transcriptMode))
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
			Phase:   m.streamPhase,
		}
		b.WriteString("\n")
		b.WriteString(RenderStreamingIndicator(m.spinnerFrame, m.agentName(), status, m.spinnerType))
	}

	// Prompt area with separator lines and project badge
	b.WriteString("\n")
	promptPath := m.projectPath
	if u, err := user.Current(); err == nil {
		promptPath = ShortenHome(promptPath, u.HomeDir)
	}
	b.WriteString(RenderPromptArea(m.inputBuf.RenderWithCursor(), promptPath, m.width))

	// Command palette (below prompt, like Claude Code)
	if m.paletteVisible && len(m.paletteFiltered) > 0 {
		b.WriteString("\n")
		b.WriteString(RenderCommandPalette(m.paletteFiltered, m.paletteSelected, m.width))
		// Pad with empty lines to maintain high-water mark height (prevents prompt bounce)
		for i := len(m.paletteFiltered); i < m.paletteMaxItems; i++ {
			b.WriteString("\n")
		}
	}

	// Model picker (below prompt)
	if m.modelPickerVisible {
		b.WriteString("\n")
		b.WriteString(RenderModelPicker(m.GetModelTiers(), m.modelPickerIdx, m.modelName, m.width))
	}

	// Status bar
	b.WriteString("\n")
	b.WriteString(RenderStatusBar(m.modelName, m.tokenCount, m.width))

	// Apply selection highlight (reverse video) if active
	if m.selection.Active() {
		return m.applySelectionHighlight(b.String())
	}

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
		if entry.Content == "" {
			return header
		}
		if entry.Status == AgentWaiting {
			// Ask-user questions: readable text with indentation for multi-line
			lines := strings.Split(entry.Content, "\n")
			first := header + " " + AgentBody.Render(lines[0])
			if len(lines) == 1 {
				return first
			}
			indent := "    "
			for _, line := range lines[1:] {
				first += "\n" + indent + AgentBody.Render(line)
			}
			return first
		}
		return header + " " + DimStyle.Render(entry.Content)

	default:
		return ""
	}
}

// Chat returns all chat entries (for testing).
func (m Model) Chat() []ChatEntry {
	return m.chat
}

// IsTranscriptMode returns whether transcript mode is active (for testing).
func (m Model) IsTranscriptMode() bool {
	return m.transcriptMode
}

// TranscriptSearchQuery returns the current transcript search query (for testing).
func (m Model) TranscriptSearchQuery() string {
	return m.tsSearchQuery
}

// TranscriptMatches returns the current search match indices (for testing).
func (m Model) TranscriptMatches() []int {
	return m.tsMatches
}

// TranscriptMatchIdx returns the current match cursor index (for testing).
func (m Model) TranscriptMatchIdx() int {
	return m.tsMatchIdx
}

// handleTranscriptSearchKey handles keystrokes while typing a search query.
func (m Model) handleTranscriptSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.tsSearching = false
		blocks := BuildTranscriptBlocks(m.chat)
		m.tsMatches = SearchTranscriptBlocks(blocks, m.tsSearchQuery)
		m.tsMatchIdx = 0
		if len(m.tsMatches) > 0 {
			m.scrollToTranscriptMatch()
		}
	case "esc":
		m.tsSearching = false
		m.tsSearchQuery = ""
		m.tsMatches = nil
	case "backspace":
		if len(m.tsSearchQuery) > 0 {
			m.tsSearchQuery = m.tsSearchQuery[:len(m.tsSearchQuery)-1]
		}
	default:
		// Only accept printable runes
		for _, r := range msg.String() {
			if r >= ' ' {
				m.tsSearchQuery += string(r)
			}
		}
	}
	return m, nil
}

// scrollToTranscriptMatch scrolls the view to center the current search match.
func (m *Model) scrollToTranscriptMatch() {
	if len(m.tsMatches) == 0 {
		return
	}
	// Approximate: scroll to bottom then let the user navigate
	// A proper implementation would compute the line offset of the matched block,
	// but that requires rendering. For now, scroll to bottom as a simple approach.
	m.scrollToBottom()
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

// HasSelection returns whether a text selection is active (for testing).
func (m Model) HasSelection() bool {
	return m.selection.Active()
}

// HasPendingQuestion returns whether the model is waiting for a user answer (for testing).
func (m Model) HasPendingQuestion() bool {
	return m.pendingQuestion != nil
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
