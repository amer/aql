package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amer/aql/internal/domain"
)

// toolDisplayNames maps internal tool names to display-friendly names.
var toolDisplayNames = map[string]string{
	"read_file":      "Read",
	"write_file":     "Write",
	"edit":           "Update",
	"list_directory": "List",
	"bash":           "Bash",
	"grep":           "Grep",
	"glob":           "Glob",
	"web_fetch":      "Fetch",
	"web_search":     "Search",
	"ask_user":       "Ask",
}

// ToolDisplayName returns a display-friendly name for a tool.
func ToolDisplayName(name string) string {
	if dn, ok := toolDisplayNames[name]; ok {
		return dn
	}
	return name
}

// toolInputExtractors defines which JSON fields to extract for each tool's header.
var toolInputExtractors = map[string]func(map[string]any) string{
	"read_file":      extractField("path"),
	"write_file":     extractField("path"),
	"edit":           extractField("path"),
	"list_directory": extractField("path"),
	"bash":           extractField("command"),
	"glob":           extractField("pattern"),
	"web_fetch":      extractField("url"),
	"web_search":     quoteField("query"),
	"ask_user":       quoteField("question"),
	"grep": func(m map[string]any) string {
		pattern, _ := m["pattern"].(string)
		path, _ := m["path"].(string)
		if pattern == "" {
			return ""
		}
		if path != "" {
			return fmt.Sprintf(`"%s", %s`, pattern, path)
		}
		return fmt.Sprintf(`"%s"`, pattern)
	},
}

func extractField(field string) func(map[string]any) string {
	return func(m map[string]any) string {
		v, _ := m[field].(string)
		return v
	}
}

func quoteField(field string) func(map[string]any) string {
	return func(m map[string]any) string {
		v, _ := m[field].(string)
		if v == "" {
			return ""
		}
		return fmt.Sprintf(`"%s"`, v)
	}
}

const maxHeaderLen = 100

// FormatToolHeader returns a display-friendly tool header like "Read(internal/tui/app.go)".
func FormatToolHeader(name, jsonInput string) string {
	displayName := ToolDisplayName(name)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(jsonInput), &parsed); err != nil {
		return displayName
	}

	extractor, ok := toolInputExtractors[name]
	if !ok {
		return displayName
	}

	arg := extractor(parsed)
	if arg == "" {
		return displayName
	}

	header := fmt.Sprintf("%s(%s)", displayName, arg)

	if len(header) > maxHeaderLen {
		// Truncate the arg portion, keeping the closing paren
		maxArg := maxHeaderLen - len(displayName) - 5 // room for "(...)"
		if maxArg > 0 {
			header = fmt.Sprintf("%s(%s...)", displayName, arg[:maxArg])
		}
	}

	return header
}

// FormatToolSummary returns a one-line summary of a tool's output.
func FormatToolSummary(name, output string, isError bool) string {
	if isError {
		first, _, _ := strings.Cut(output, "\n")
		if first == "" {
			return "error"
		}
		return first
	}

	if output == "" {
		return "(no output)"
	}

	switch name {
	case "read_file":
		n := countLines(output)
		return pluralize(n, "line", "lines")
	case "write_file":
		n := countLines(output)
		return pluralize(n, "line", "lines")
	case "list_directory":
		n := countNonEmptyLines(output)
		return pluralize(n, "item", "items")
	case "bash":
		first, _, _ := strings.Cut(output, "\n")
		if len(first) > 80 {
			return first[:77] + "..."
		}
		return first
	default:
		first, _, _ := strings.Cut(output, "\n")
		if len(first) > 80 {
			return first[:77] + "..."
		}
		return first
	}
}

func countLines(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func countNonEmptyLines(s string) int {
	n := 0
	for line := range strings.SplitSeq(s, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// --- TranscriptBlock types and grouping ---

// BlockType identifies the kind of transcript block.
type BlockType int

const (
	BlockUser      BlockType = iota // user input
	BlockAssistant                  // assistant response (text + tools)
	BlockStatus                     // standalone status message
)

// ToolEntry holds a tool call paired with its result.
type ToolEntry struct {
	Call   domain.ToolCall  // The Running-phase entry (Name, ToolID, Input JSON)
	Result *domain.ToolCall // The Done/Error-phase entry (nil if still running)
}

// TranscriptBlock represents a top-level block in the transcript.
type TranscriptBlock struct {
	Type       BlockType
	AgentName  string
	TextParts  []string
	Tools      []ToolEntry
	StatusText string
}

// ToolGroup represents consecutive tool calls of the same type.
type ToolGroup struct {
	DisplayName string
	Entries     []ToolEntry
}

// BuildTranscriptBlocks groups a flat []ChatEntry into []TranscriptBlock.
func BuildTranscriptBlocks(entries []ChatEntry) []TranscriptBlock {
	if len(entries) == 0 {
		return nil
	}

	var blocks []TranscriptBlock

	for _, entry := range entries {
		switch entry.Type {
		case EntryUserInput:
			blocks = append(blocks, TranscriptBlock{
				Type:      BlockUser,
				TextParts: []string{entry.Content},
			})

		case EntryAgentText:
			cur := currentAssistantBlock(&blocks, entry.AgentName)
			cur.TextParts = append(cur.TextParts, entry.Content)

		case EntryAgentTool:
			if entry.ToolCall == nil {
				continue
			}
			tc := *entry.ToolCall
			// Try to merge with an existing running tool with the same ToolID
			cur := currentAssistantBlock(&blocks, entry.AgentName)
			if tc.ToolID != "" && tc.Status != domain.ToolRunning {
				if merged := mergeToolResult(cur, tc); merged {
					continue
				}
			}
			cur.Tools = append(cur.Tools, ToolEntry{Call: tc})

		case EntryAgentStatus:
			if entry.Status == AgentDone {
				// Attach to preceding assistant block if possible
				if len(blocks) > 0 && blocks[len(blocks)-1].Type == BlockAssistant {
					blocks[len(blocks)-1].StatusText = entry.Content
					continue
				}
			}
			blocks = append(blocks, TranscriptBlock{
				Type:       BlockStatus,
				AgentName:  entry.AgentName,
				StatusText: entry.Content,
			})
		}
	}

	return blocks
}

// currentAssistantBlock returns a pointer to the last block if it's an assistant
// block for the same agent, or appends a new one.
func currentAssistantBlock(blocks *[]TranscriptBlock, agentName string) *TranscriptBlock {
	if len(*blocks) > 0 {
		last := &(*blocks)[len(*blocks)-1]
		if last.Type == BlockAssistant && last.AgentName == agentName {
			return last
		}
	}
	*blocks = append(*blocks, TranscriptBlock{
		Type:      BlockAssistant,
		AgentName: agentName,
	})
	return &(*blocks)[len(*blocks)-1]
}

// mergeToolResult finds a running tool with matching ToolID and sets its result.
func mergeToolResult(block *TranscriptBlock, done domain.ToolCall) bool {
	for i := len(block.Tools) - 1; i >= 0; i-- {
		if block.Tools[i].Call.ToolID == done.ToolID && block.Tools[i].Result == nil {
			block.Tools[i].Result = &done
			return true
		}
	}
	return false
}

// GroupConsecutiveTools groups consecutive same-type tool entries.
func GroupConsecutiveTools(tools []ToolEntry) []ToolGroup {
	if len(tools) == 0 {
		return nil
	}

	var groups []ToolGroup
	current := ToolGroup{
		DisplayName: ToolDisplayName(tools[0].Call.Name),
		Entries:     []ToolEntry{tools[0]},
	}

	for _, te := range tools[1:] {
		dn := ToolDisplayName(te.Call.Name)
		if dn == current.DisplayName {
			current.Entries = append(current.Entries, te)
		} else {
			groups = append(groups, current)
			current = ToolGroup{
				DisplayName: dn,
				Entries:     []ToolEntry{te},
			}
		}
	}
	groups = append(groups, current)

	return groups
}

// --- Transcript rendering ---

const (
	transcriptMarker    = "⏺"
	transcriptConnector = "⎿"
	markerWidth         = 1 // display width of marker/connector in terminal cells
	gapWidth            = 1 // spaces between marker and text
)

var (
	transcriptPadding = strings.Repeat(" ", gapWidth)
	transcriptIndent  = strings.Repeat(" ", markerWidth+gapWidth)
)

// MarkerState represents the visual state of a ⏺ marker.
type MarkerState int

const (
	MarkerActive  MarkerState = iota // default: assistant text (brand/orange)
	MarkerRunning                    // tool in progress (warning/yellow)
	MarkerDone                       // tool completed (success/green)
	MarkerError                      // tool failed (error/red)
)

// StyledMarker returns the ⏺ character styled for the given state.
func StyledMarker(state MarkerState) string {
	switch state {
	case MarkerRunning:
		return TranscriptMarkerRunning.Render(transcriptMarker)
	case MarkerDone:
		return TranscriptMarkerDone.Render(transcriptMarker)
	case MarkerError:
		return TranscriptMarkerError.Render(transcriptMarker)
	default:
		return TranscriptMarkerActive.Render(transcriptMarker)
	}
}

// RenderTranscriptBlock renders a single transcript block in Claude Code style.
func RenderTranscriptBlock(block TranscriptBlock, width int, expanded bool) string {
	switch block.Type {
	case BlockUser:
		return RenderUserMessage(strings.Join(block.TextParts, ""))
	case BlockStatus:
		return transcriptIndent + DimStyle.Render(block.StatusText) + "\n"
	case BlockAssistant:
		return renderAssistantBlock(block, width, expanded)
	default:
		return ""
	}
}

func renderAssistantBlock(block TranscriptBlock, width int, expanded bool) string {
	var b strings.Builder
	contentWidth := width - len(transcriptIndent) // account for marker + padding

	// Render text parts
	for _, text := range block.TextParts {
		rendered := RenderMarkdown(text, contentWidth)
		if rendered == "" {
			rendered = AgentBody.Render(text)
		}
		b.WriteString(StyledMarker(MarkerActive))
		b.WriteString(transcriptPadding)
		// Indent continuation lines
		lines := strings.Split(rendered, "\n")
		for i, line := range lines {
			if i == 0 {
				b.WriteString(line)
			} else {
				b.WriteString("\n" + transcriptIndent + line)
			}
		}
		b.WriteString("\n")
	}

	// Render tools (grouped)
	groups := GroupConsecutiveTools(block.Tools)
	for _, group := range groups {
		b.WriteString(renderToolGroup(group, width, expanded))
	}

	// Render status text (completion indicator)
	if block.StatusText != "" {
		b.WriteString("\n" + transcriptIndent + DimStyle.Render(block.StatusText) + "\n")
	}

	return b.String()
}

func renderToolGroup(group ToolGroup, width int, expanded bool) string {
	// Groups of 3+ collapse when not expanded
	if len(group.Entries) >= 3 && !expanded {
		return renderCollapsedToolGroup(group, width)
	}

	// Render each tool individually
	var b strings.Builder
	for _, entry := range group.Entries {
		b.WriteString(renderToolEntry(entry, width, expanded))
	}
	return b.String()
}

// allToolsDone returns true if every entry in the group has a result.
func allToolsDone(entries []ToolEntry) bool {
	for _, e := range entries {
		if e.Result == nil {
			return false
		}
	}
	return true
}

// collectToolPaths extracts file paths from each tool entry's JSON input.
func collectToolPaths(entries []ToolEntry) []string {
	var paths []string
	for _, e := range entries {
		if path := extractPathFromInput(e.Call.Name, e.Call.Content); path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

func renderCollapsedToolGroup(group ToolGroup, width int) string {
	var b strings.Builder
	done := allToolsDone(group.Entries)

	header := fmt.Sprintf("%s %d files", group.DisplayName, len(group.Entries))
	if !done {
		header = fmt.Sprintf("%sing %d files...", group.DisplayName, len(group.Entries))
	}

	b.WriteString("\n")
	state := MarkerRunning
	if done {
		state = MarkerDone
	}
	b.WriteString(StyledMarker(state))
	b.WriteString(" ")
	b.WriteString(ToolHeaderStyle.Render(header))
	b.WriteString("\n")

	paths := collectToolPaths(group.Entries)
	if len(paths) > 0 {
		b.WriteString(renderConnectorLine(strings.Join(paths, ", "), width))
	}
	b.WriteString("\n")

	return b.String()
}

// renderConnectorLine renders a ⎿ connector with truncated summary text.
func renderConnectorLine(summary string, width int) string {
	maxLen := width - 8
	if maxLen > 0 && len(summary) > maxLen {
		summary = summary[:maxLen-6] + ", +more"
	}
	return transcriptIndent +
		TranscriptConnectorStyle.Render(transcriptConnector) +
		"  " + DimStyle.Render(summary)
}

func renderToolEntry(entry ToolEntry, width int, expanded bool) string {
	header := FormatToolHeader(entry.Call.Name, entry.Call.Content)

	if entry.Result == nil {
		return renderRunningTool(header)
	}
	return renderCompletedTool(entry, header, expanded)
}

func renderRunningTool(header string) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(StyledMarker(MarkerRunning))
	b.WriteString(" ")
	b.WriteString(ToolHeaderStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(transcriptIndent)
	b.WriteString(TranscriptConnectorStyle.Render(transcriptConnector))
	b.WriteString("  ")
	b.WriteString(ToolStatusRunning.Render("⟳ running..."))
	b.WriteString("\n")
	return b.String()
}

func renderCompletedTool(entry ToolEntry, header string, expanded bool) string {
	var b strings.Builder
	isError := entry.Result.Status == domain.ToolError

	b.WriteString("\n")
	if isError {
		b.WriteString(StyledMarker(MarkerError))
	} else {
		b.WriteString(StyledMarker(MarkerDone))
	}
	b.WriteString(" ")
	b.WriteString(ToolHeaderStyle.Render(header))
	b.WriteString("\n")

	b.WriteString(transcriptIndent)
	b.WriteString(TranscriptConnectorStyle.Render(transcriptConnector))
	b.WriteString("  ")
	if expanded {
		output := entry.Result.Content
		if output == "" {
			output = "(no output)"
		}
		b.WriteString(ToolContentStyle.Render(output))
	} else {
		summary := FormatToolSummary(entry.Call.Name, entry.Result.Content, isError)
		b.WriteString(DimStyle.Render(summary))
	}
	b.WriteString("\n")

	return b.String()
}

// extractPathFromInput extracts a file path from a tool's JSON input.
func extractPathFromInput(toolName, jsonInput string) string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(jsonInput), &parsed); err != nil {
		return ""
	}
	switch toolName {
	case "read_file", "write_file", "edit", "list_directory":
		v, _ := parsed["path"].(string)
		return v
	case "bash":
		v, _ := parsed["command"].(string)
		return v
	case "grep", "glob":
		v, _ := parsed["pattern"].(string)
		return v
	default:
		return ""
	}
}

// --- Transcript search ---

// SearchTranscriptBlocks returns indices of blocks containing the query (case-insensitive).
func SearchTranscriptBlocks(blocks []TranscriptBlock, query string) []int {
	if query == "" {
		return nil
	}
	q := strings.ToLower(query)
	var matches []int

	for i, block := range blocks {
		if blockContains(block, q) {
			matches = append(matches, i)
		}
	}
	return matches
}

func blockContains(block TranscriptBlock, lowerQuery string) bool {
	for _, text := range block.TextParts {
		if strings.Contains(strings.ToLower(text), lowerQuery) {
			return true
		}
	}
	if strings.Contains(strings.ToLower(block.StatusText), lowerQuery) {
		return true
	}
	for _, tool := range block.Tools {
		if strings.Contains(strings.ToLower(tool.Call.Name), lowerQuery) {
			return true
		}
		if strings.Contains(strings.ToLower(tool.Call.Content), lowerQuery) {
			return true
		}
		if tool.Result != nil && strings.Contains(strings.ToLower(tool.Result.Content), lowerQuery) {
			return true
		}
	}
	return false
}
