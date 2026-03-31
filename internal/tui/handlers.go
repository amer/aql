package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

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
	// Transcript search mode intercept
	if m.tsSearching {
		return m.handleTranscriptSearchKey(msg)
	}
	if m.transcriptMode {
		if model, cmd, handled := m.handleTranscriptModeKey(msg); handled {
			return model, cmd
		}
	}

	switch msg.String() {
	case "ctrl+c", "ctrl+d":
		if m.streaming && m.cancelStream != nil {
			m.cancelStream()
		}
		return m, tea.Quit
	case "esc":
		m.handleEscKey()
	case "tab":
		m.handleTabComplete()
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
	case "ctrl+o":
		m.transcriptMode = !m.transcriptMode
	case "shift+up":
		m.scrollUp(3)
	case "shift+down":
		m.scrollDown(3)
	case "pgup":
		m.scrollUp(m.visibleHeight() / 2)
	case "pgdown":
		m.scrollDown(m.visibleHeight() / 2)
	case "up":
		m.handleUpKey()
	case "down":
		m.handleDownKey()
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

// handleTranscriptModeKey handles keys specific to transcript mode.
// Returns handled=true if the key was consumed.
func (m Model) handleTranscriptModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "/":
		m.tsSearching = true
		m.tsSearchQuery = ""
		return m, nil, true
	case "n":
		if len(m.tsMatches) > 0 {
			m.tsMatchIdx = (m.tsMatchIdx + 1) % len(m.tsMatches)
			m.scrollToTranscriptMatch()
		}
		return m, nil, true
	case "N":
		if len(m.tsMatches) > 0 {
			m.tsMatchIdx = (m.tsMatchIdx - 1 + len(m.tsMatches)) % len(m.tsMatches)
			m.scrollToTranscriptMatch()
		}
		return m, nil, true
	case "esc":
		m.transcriptMode = false
		m.tsSearchQuery = ""
		m.tsMatches = nil
		m.tsMatchIdx = 0
		return m, nil, true
	}
	return m, nil, false
}

func (m *Model) handleEscKey() {
	if m.paletteVisible {
		m.paletteVisible = false
		m.paletteSelected = 0
		m.inputBuf.Clear()
	} else if m.streaming {
		// Esc during streaming cancels the in-flight API call and stays
		// in the app — unlike Ctrl+C which quits entirely. This lets users
		// stop a runaway response without losing their session.
		if m.cancelStream != nil {
			m.cancelStream()
		}
		m.streaming = false
		m.chat = append(m.chat, ChatEntry{
			Type:    EntryAgentStatus,
			Content: "Interrupted",
			Status:  AgentError,
		})
		m.autoScroll()
	}
}

func (m *Model) handleTabComplete() {
	if m.paletteVisible && len(m.paletteFiltered) > 0 {
		m.inputBuf.Set(m.paletteFiltered[m.paletteSelected].Name)
		m.paletteVisible = false
		m.paletteSelected = 0
	}
}

func (m *Model) handleUpKey() {
	if m.paletteVisible {
		if m.paletteSelected > 0 {
			m.paletteSelected--
		}
	} else if val, ok := m.history.Previous(); ok {
		m.inputBuf.Set(val)
	}
}

func (m *Model) handleDownKey() {
	if m.paletteVisible {
		if m.paletteSelected < len(m.paletteFiltered)-1 {
			m.paletteSelected++
		}
	} else if val, ok := m.history.Next(); ok {
		m.inputBuf.Set(val)
	}
}

func (m Model) handleSubmit() (tea.Model, tea.Cmd) {
	input := m.inputBuf.String()
	if input == "" {
		return m, nil
	}

	// Resolve command: palette selection takes priority.
	cmd := strings.TrimSpace(input)
	if m.paletteVisible && len(m.paletteFiltered) > 0 {
		cmd = m.paletteFiltered[m.paletteSelected].Name
	}
	m.paletteVisible = false
	m.paletteSelected = 0

	// Exit commands always work, even during streaming.
	if cmd == "/exit" || cmd == "/quit" || cmd == "/q" {
		if m.streaming && m.cancelStream != nil {
			m.cancelStream()
		}
		return m, tea.Quit
	}

	// Answer a pending ask_user question — always allowed, even during streaming.
	if m.pendingQuestion != nil {
		return m.handleAnswerQuestion(cmd)
	}

	if m.streaming {
		return m, nil
	}
	if result, resultCmd := m.executeCommand(cmd); result != "" {
		return m, resultCmd
	}

	if IsBashCommand(input) {
		return m.handleBashSubmit(input)
	}

	return m.handleNormalSubmit(input)
}

func (m Model) handleAnswerQuestion(answer string) (tea.Model, tea.Cmd) {
	answer = strings.TrimSpace(answer)
	m.chat = append(m.chat, ChatEntry{Type: EntryUserInput, Content: answer})
	m.inputBuf.Clear()
	m.scrollToBottom()
	responseCh := m.pendingQuestion.ResponseCh
	m.pendingQuestion = nil
	go func() { responseCh <- answer }()
	return m, nil
}

func (m Model) handleBashSubmit(input string) (tea.Model, tea.Cmd) {
	shellCmd := ParseBashCommand(input)
	if shellCmd == "" {
		return m, nil
	}
	m.history.Push(input)
	m.chat = append(m.chat, ChatEntry{Type: EntryUserInput, Content: input})
	m.inputBuf.Clear()
	m.scrollToBottom()
	if m.onBash != nil {
		return m, m.onBash(shellCmd)
	}
	return m, nil
}

func (m Model) handleNormalSubmit(input string) (tea.Model, tea.Cmd) {
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
		return m.handleSpinnerTick()
	case TokenCountMsg:
		m.tokenCount = msg.Count
	case AgentStreamStartMsg:
		return m.handleStreamStart()
	case AgentStreamDeltaMsg:
		return m.handleStreamDelta(msg)
	case TokenUsageMsg:
		m.tokenCount = msg.InputTokens + msg.OutputTokens
	case AgentStreamDoneMsg:
		m.handleStreamDone()
	case AgentStreamErrorMsg:
		m.handleStreamError(msg)
	case AgentOutputMsg:
		m.handleAgentOutput(msg)
	case AgentStatusMsg:
		m.handleAgentStatus(msg)
	case CompactDoneMsg:
		m.handleCompactDone(msg)
	case BashResultMsg:
		m.handleBashResult(msg)
	case ModelsLoadedMsg:
		if len(msg.Tiers) > 0 {
			m.modelTiers = msg.Tiers
		}
	case ModelSelectedMsg:
		if m.onModelSelected != nil {
			m.onModelSelected(msg.Model)
		}
	case AgentToolCallMsg:
		m.handleToolCall(msg)
	case AgentAskUserMsg:
		m.handleAskUser(msg)
	}
	return m, nil
}

func (m Model) handleSpinnerTick() (tea.Model, tea.Cmd) {
	if m.streaming {
		m.spinnerFrame++
		return m, SpinnerTickFor(m.spinnerType)
	}
	return m, nil
}

func (m Model) handleStreamStart() (tea.Model, tea.Cmd) {
	if !m.streaming {
		m.startStream()
		m.streamPhase = PhaseRequesting
		return m, SpinnerTickFor(m.spinnerType)
	}
	return m, nil
}

func (m Model) handleStreamDelta(msg AgentStreamDeltaMsg) (tea.Model, tea.Cmd) {
	wasStreaming := m.streaming
	if !wasStreaming {
		m.startStream()
	}
	m.streamPhase = PhaseResponding
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
	return m, nil
}

func (m *Model) handleStreamDone() {
	elapsed := time.Since(m.streamStart)
	m.streaming = false
	m.chat = append(m.chat, ChatEntry{
		Type:    EntryAgentStatus,
		Content: "\n" + RenderCompletionIndicator(elapsed) + "\n",
		Status:  AgentDone,
	})
	m.autoScroll()
}

func (m *Model) handleStreamError(msg AgentStreamErrorMsg) {
	m.streaming = false
	m.chat = append(m.chat, ChatEntry{
		Type:      EntryAgentStatus,
		AgentName: msg.AgentName,
		Status:    AgentError,
		Content:   msg.Error.Error(),
	})
	m.autoScroll()
}

func (m *Model) handleAgentOutput(msg AgentOutputMsg) {
	m.chat = append(m.chat, ChatEntry{
		Type:      EntryAgentText,
		AgentName: msg.AgentName,
		Content:   msg.Output,
	})
	m.autoScroll()
}

func (m *Model) handleAgentStatus(msg AgentStatusMsg) {
	m.chat = append(m.chat, ChatEntry{
		Type:      EntryAgentStatus,
		AgentName: msg.AgentName,
		Content:   msg.StatusMsg,
		Status:    msg.Status,
	})
	m.autoScroll()
}

func (m *Model) handleCompactDone(msg CompactDoneMsg) {
	m.streaming = false
	if msg.Err != nil {
		m.addStatusChat("Compact failed: "+msg.Err.Error(), AgentError)
	} else {
		m.chat = nil
		m.addStatusChat("Conversation compacted", AgentDone)
		// After compaction, estimate from summary until next API call provides precise counts
		m.tokenCount = EstimateTokens(len(msg.Summary))
	}
	m.autoScroll()
}

func (m *Model) handleBashResult(msg BashResultMsg) {
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
}

func (m *Model) handleToolCall(msg AgentToolCallMsg) {
	m.chat = append(m.chat, ChatEntry{
		Type:      EntryAgentTool,
		AgentName: msg.AgentName,
		ToolCall:  &msg.ToolCall,
	})
	m.autoScroll()
}

func (m *Model) handleAskUser(msg AgentAskUserMsg) {
	m.pendingQuestion = &msg
	m.chat = append(m.chat, ChatEntry{
		Type:    EntryAgentStatus,
		Content: msg.Question,
		Status:  AgentWaiting,
	})
	m.autoScroll()
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
