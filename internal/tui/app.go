package tui

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Model struct (main Bubble Tea model), NewModel constructor,
//     Init/Update/View methods, callback setters (SetOnBash,
//     SetCancelStream, SetOnClear, SetOnCompact, SetOnModelSelected,
//     SetModelTiers, etc.), scroll management, command palette logic,
//     executeCommand handler, View rendering,
//     testing accessors (Chat, IsStreaming, HasSelection, etc.)
//
// MUST NOT GO HERE:
//   - Agent or LLM imports (TUI never imports agent), tool execution,
//     direct API calls, creating agents. Communication is via
//     callbacks and message types only.
//
// Q: Should I add a new slash command?
// A: Add it to commands.go's SlashCommands() and handle it in
//    executeCommand() here.
//
// Q: Should I add a new callback?
// A: Add a Set* method here and call it from cmd/aql/main.go.
//
// Q: How do I access agent state from the TUI?
// A: You don't. Use callbacks injected via Set*() methods. The TUI
//    knows about messages, not agents.
// ──────────────────────────────────────────────────────────────────

import (
	"fmt"
	"os/user"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Version is the application version, set at build time via ldflags.
var Version = "dev"

// streamState tracks the state of an in-progress API streaming response.
type streamState struct {
	active       bool
	spinnerFrame int
	spinnerType  SpinnerType
	start        time.Time   // when current streaming started
	chars        int         // chars received in current stream
	phase        StreamPhase // current streaming phase (requesting/responding)
	cancel       func()      // cancels the in-flight API call context
}

// modelPickerState tracks the model picker overlay.
type modelPickerState struct {
	visible bool
	idx     int
	input   string
	tiers   []ModelTier // dynamic model tiers from API; nil = use defaults
}

// transcriptSearchState tracks Ctrl+O transcript mode and search.
type transcriptSearchState struct {
	mode      bool   // Ctrl+O: expand all tools, enable search
	searching bool   // actively typing a search query
	query     string // current transcript search query
	matches   []int  // block indices matching the search
	matchIdx  int    // current match index
}

// paletteState tracks the command palette overlay.
type paletteState struct {
	visible  bool
	selected int
	filtered []Command
	maxItems int // high-water mark: max items shown during this palette session
}

// Model is the main Bubble Tea model for AQL.
type Model struct {
	workflowName    string
	agentNames      []string
	chat            []ChatEntry
	inputBuf        *InputBuffer
	history         *History
	width           int
	height          int
	scrollOffset    int
	tokenCount      int
	modelName       string
	projectPath     string
	selection       Selection        // mouse text selection state
	viewLines       []string         // plain text lines from last render (for selection extraction)
	pendingQuestion *AgentAskUserMsg // non-nil when agent is waiting for user answer

	stream    streamState
	picker    modelPickerState
	tsSearch  transcriptSearchState
	palette   paletteState
	taskPanel taskState
	diffPanel diffState

	// Callbacks
	onSubmit        SubmitFunc
	onBash          BashFunc
	onModelSelected func(modelID string)
	onClear         func()         // called on /clear to reset agent context
	onCompact       func() tea.Cmd // called on /compact to summarize context
	onDiff          func() tea.Cmd // called on /diff to fetch git diff
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
	m.stream.cancel = fn
}

// SetOnClear sets the function called when /clear is executed to reset agent context.
func (m *Model) SetOnClear(fn func()) {
	m.onClear = fn
}

// SetOnCompact sets the function called when /compact is executed to summarize context.
func (m *Model) SetOnCompact(fn func() tea.Cmd) {
	m.onCompact = fn
}

// SetOnDiff sets the callback for fetching git diff data.
func (m *Model) SetOnDiff(fn func() tea.Cmd) {
	m.onDiff = fn
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
	m.picker.tiers = tiers
}

// GetModelTiers returns the current model tiers, falling back to defaults.
func (m Model) GetModelTiers() []ModelTier {
	if len(m.picker.tiers) > 0 {
		return m.picker.tiers
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
	m.stream.spinnerType = st
}

// ActiveSpinnerType returns the current spinner animation style.
func (m Model) ActiveSpinnerType() SpinnerType {
	return m.stream.spinnerType
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.diffPanel.visible {
			return m.handleDiffKey(msg)
		}
		if m.picker.visible {
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
			m.selection.Clear()
			m.computeViewLines() // snapshot screen content for selection extraction
			m.selection.Start(msg.X, msg.Y)
		case msg.Action == tea.MouseActionMotion && m.selection.Active():
			m.selection.Update(msg.X, msg.Y)
		case msg.Action == tea.MouseActionRelease && m.selection.Active():
			m.selection.Update(msg.X, msg.Y)
			text := m.selection.Extract(m.viewLines)
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
	if m.stream.active {
		reservedLines++ // streaming indicator (\n + text)
	}
	h := max(m.height-reservedLines, 1)
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
		m.palette.visible = true
		m.palette.filtered = FilterCommands(SlashCommands(), input)
		if m.palette.selected >= len(m.palette.filtered) {
			m.palette.selected = 0
		}
		// Track high-water mark so the palette area never shrinks while typing
		if len(m.palette.filtered) > m.palette.maxItems {
			m.palette.maxItems = len(m.palette.filtered)
		}
	} else {
		m.palette.visible = false
		m.palette.selected = 0
		m.palette.maxItems = 0
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
		m.showHelp()
		return "help", nil
	case "/agents":
		m.addStatusChat("Active agents: "+strings.Join(m.agentNames, ", "), AgentActive)
		return "agents", nil
	case "/status":
		m.addStatusChat("Workflow: "+m.workflowName+" · Agents: "+strings.Join(m.agentNames, ", "), AgentActive)
		return "status", nil
	case "/model":
		m.openModelPicker()
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
		m.stream.phase = PhaseRequesting
		return "compact", m.onCompact()
	case "/tasks":
		m.toggleTaskPanel()
		return "tasks", nil
	case "/diff":
		if m.onDiff == nil {
			m.addStatusChat("Diff is not available", AgentError)
			return "diff", nil
		}
		m.diffPanel.loading = true
		m.inputBuf.Clear()
		return "diff", m.onDiff()
	case "/spinner":
		m.cycleSpinner()
		return "spinner", nil
	}
	return "", nil
}

// showHelp appends the list of available slash commands to the transcript.
func (m *Model) showHelp() {
	lines := []string{"Available commands:"}
	for _, c := range SlashCommands() {
		lines = append(lines, "  "+c.Name+" — "+c.Description)
	}
	m.addStatusChat(strings.Join(lines, "\n"), AgentActive)
}

// toggleTaskPanel flips task-panel visibility, or reports when none exist.
func (m *Model) toggleTaskPanel() {
	if len(m.taskPanel.tasks) == 0 {
		m.addStatusChat("No tasks tracked yet", AgentActive)
		return
	}
	m.taskPanel.visible = !m.taskPanel.visible
	if m.taskPanel.visible {
		m.addStatusChat("Task panel shown (ctrl+t to toggle)", AgentActive)
	} else {
		m.addStatusChat("Task panel hidden (ctrl+t to toggle)", AgentActive)
	}
}

// cycleSpinner advances the streaming spinner to the next style.
func (m *Model) cycleSpinner() {
	types := SpinnerTypes()
	next := SpinnerBraille
	for i, st := range types {
		if st == m.stream.spinnerType {
			next = types[(i+1)%len(types)]
			break
		}
	}
	m.stream.spinnerType = next
	def := SpinnerDef(next)
	m.addStatusChat("Spinner: "+def.Name+" "+def.Frames[0], AgentActive)
}

// View implements tea.Model.
func (m Model) View() string {
	// Diff overlay takes over the full screen when visible.
	if m.diffPanel.visible {
		return m.renderDiffOverlay()
	}

	var b strings.Builder

	viewport, offset := m.renderTranscriptViewport()
	b.WriteString(viewport)

	// Scroll indicator when not at bottom
	if offset > 0 {
		indicator := fmt.Sprintf(" ↓ %d more lines below ", offset)
		b.WriteString("\n")
		b.WriteString(DimStyle.Render(indicator))
	}

	// Streaming indicator (directly above prompt)
	if m.stream.active {
		status := StreamStatus{
			Elapsed: time.Since(m.stream.start).Truncate(time.Second),
			Tokens:  EstimateTokens(m.stream.chars),
			Phase:   m.stream.phase,
		}
		b.WriteString("\n")
		b.WriteString(RenderStreamingIndicator(m.stream.spinnerFrame, m.agentName(), status, m.stream.spinnerType))
	}

	// Task panel (between streaming indicator and prompt)
	if m.taskPanel.visible && len(m.taskPanel.tasks) > 0 {
		panel := RenderTaskPanel(m.taskPanel.tasks, m.width)
		if panel != "" {
			b.WriteString(panel)
		}
	}

	b.WriteString(m.renderPromptSection())

	// Apply selection highlight (reverse video) if active
	if m.selection.Active() {
		return m.applySelectionHighlight(b.String())
	}

	return b.String()
}

// renderTranscriptViewport renders the welcome banner and chat transcript,
// clamps the scroll window to the visible height, and pads short
// conversations down to the prompt. It returns the rendered content and the
// clamped scroll offset (0 = at bottom) so the caller can show a scroll hint.
func (m Model) renderTranscriptViewport() (string, int) {
	chatLines := []string{RenderWelcome(m.welcomeData()), ""} // banner + blank separator
	for _, block := range BuildTranscriptBlocks(m.chat) {
		chatLines = append(chatLines, RenderTranscriptBlock(block, m.width, m.tsSearch.mode))
	}

	vh := m.visibleHeight()
	contentLines := strings.Split(strings.Join(chatLines, "\n"), "\n")

	// Clamp scrollOffset to valid range, then slice the window from the bottom.
	offset := min(m.scrollOffset, max(len(contentLines)-vh, 0))
	end := max(len(contentLines)-offset, 0)
	start := max(end-vh, 0)
	visible := contentLines[start:end]

	var b strings.Builder
	// Pad above content so short conversations sit close to the prompt.
	for range vh - len(visible) {
		b.WriteString("\n")
	}
	b.WriteString(strings.Join(visible, "\n"))
	return b.String(), offset
}

// renderPromptSection renders everything from the prompt area down: the input
// prompt, the command palette or model picker overlay, and the status bar.
func (m Model) renderPromptSection() string {
	var b strings.Builder

	// Prompt area with separator lines and project badge
	b.WriteString("\n")
	promptPath := m.projectPath
	if u, err := user.Current(); err == nil {
		promptPath = ShortenHome(promptPath, u.HomeDir)
	}
	b.WriteString(RenderPromptArea(m.inputBuf.RenderWithCursor(), promptPath, m.width))

	// Command palette (below prompt, like Claude Code)
	if m.palette.visible && len(m.palette.filtered) > 0 {
		b.WriteString("\n")
		b.WriteString(RenderCommandPalette(m.palette.filtered, m.palette.selected, m.width))
		// Pad with empty lines to maintain high-water mark height (prevents prompt bounce)
		for i := len(m.palette.filtered); i < m.palette.maxItems; i++ {
			b.WriteString("\n")
		}
	}

	// Model picker (below prompt)
	if m.picker.visible {
		b.WriteString("\n")
		b.WriteString(RenderModelPicker(m.GetModelTiers(), m.picker.idx, m.modelName, m.width))
	}

	// Status bar
	b.WriteString("\n")
	var hints []string
	if len(m.taskPanel.tasks) > 0 {
		if m.taskPanel.visible {
			hints = append(hints, "ctrl+t to hide tasks")
		} else {
			hints = append(hints, "ctrl+t to show tasks")
		}
	}
	b.WriteString(RenderStatusBar(m.modelName, m.tokenCount, m.width, hints...))

	return b.String()
}

// agentName returns the first agent name for display, or "agent".
func (m Model) agentName() string {
	if len(m.agentNames) > 0 {
		return m.agentNames[0]
	}
	return "agent"
}

// Chat returns all chat entries (for testing).
func (m Model) Chat() []ChatEntry {
	return m.chat
}

// IsTranscriptMode returns whether transcript mode is active (for testing).
func (m Model) IsTranscriptMode() bool {
	return m.tsSearch.mode
}

// TranscriptSearchQuery returns the current transcript search query (for testing).
func (m Model) TranscriptSearchQuery() string {
	return m.tsSearch.query
}

// TranscriptMatches returns the current search match indices (for testing).
func (m Model) TranscriptMatches() []int {
	return m.tsSearch.matches
}

// TranscriptMatchIdx returns the current match cursor index (for testing).
func (m Model) TranscriptMatchIdx() int {
	return m.tsSearch.matchIdx
}

// handleTranscriptSearchKey handles keystrokes while typing a search query.
func (m Model) handleTranscriptSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.tsSearch.searching = false
		blocks := BuildTranscriptBlocks(m.chat)
		m.tsSearch.matches = SearchTranscriptBlocks(blocks, m.tsSearch.query)
		m.tsSearch.matchIdx = 0
		if len(m.tsSearch.matches) > 0 {
			m.scrollToTranscriptMatch()
		}
	case "esc":
		m.tsSearch.searching = false
		m.tsSearch.query = ""
		m.tsSearch.matches = nil
	case "backspace":
		if len(m.tsSearch.query) > 0 {
			m.tsSearch.query = m.tsSearch.query[:len(m.tsSearch.query)-1]
		}
	default:
		// Only accept printable runes
		for _, r := range msg.String() {
			if r >= ' ' {
				m.tsSearch.query += string(r)
			}
		}
	}
	return m, nil
}

// scrollToTranscriptMatch scrolls the view to center the current search match.
func (m *Model) scrollToTranscriptMatch() {
	if len(m.tsSearch.matches) == 0 {
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
	return m.stream.active
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
	return m.palette.visible
}

// PaletteSelected returns the currently selected palette index (for testing).
func (m Model) PaletteSelected() int {
	return m.palette.selected
}

// PaletteCommands returns the currently filtered palette commands (for testing).
func (m Model) PaletteCommands() []Command {
	return m.palette.filtered
}

// IsModelPickerVisible returns whether the model picker is visible (for testing).
func (m Model) IsModelPickerVisible() bool {
	return m.picker.visible
}

// ModelPickerSelected returns the currently selected picker index (for testing).
func (m Model) ModelPickerSelected() int {
	return m.picker.idx
}
