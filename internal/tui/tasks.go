package tui

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - TaskEntry struct, taskState, isTaskTool — identifies task
//     tools for transcript suppression, handleTaskToolResult —
//     parses task output and updates panel state, upsertTask,
//     ToggleTasks, task panel rendering (taskCheckbox,
//     taskDescriptionStyle, RenderTaskPanel), testing accessors.
//
// MUST NOT GO HERE:
//   - TaskStore implementation (tools/task.go), agent imports, tool
//     definitions.
//
// Q: Should I suppress a new tool from the transcript?
// A: If it's task-like, add it to isTaskTool() and handle in
//    handleTaskToolResult().
// ──────────────────────────────────────────────────────────────────

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TaskEntry represents a tracked task for display in the TUI.
type TaskEntry struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
	ChangedAt   time.Time
}

// taskState holds the TUI-side task tracking state.
type taskState struct {
	tasks   []TaskEntry
	visible bool
}

// isTaskTool returns true if the tool name is a task tracking tool.
func isTaskTool(name string) bool {
	switch name {
	case "task_create", "task_update", "task_list":
		return true
	}
	return false
}

// handleTaskToolResult parses a task tool's done output and updates task state.
// Returns true if the event was handled (and should be suppressed from transcript).
func (m *Model) handleTaskToolResult(toolName, output string) bool {
	switch toolName {
	case "task_create":
		var task TaskEntry
		if err := json.Unmarshal([]byte(output), &task); err != nil {
			return true
		}
		task.ChangedAt = time.Now()
		m.taskPanel.tasks = upsertTask(m.taskPanel.tasks, task)
		m.taskPanel.visible = true
	case "task_update":
		var task TaskEntry
		if err := json.Unmarshal([]byte(output), &task); err != nil {
			return true
		}
		task.ChangedAt = time.Now()
		m.taskPanel.tasks = upsertTask(m.taskPanel.tasks, task)
	case "task_list":
		var tasks []TaskEntry
		if err := json.Unmarshal([]byte(output), &tasks); err != nil {
			return true
		}
		now := time.Now()
		for i := range tasks {
			tasks[i].ChangedAt = now
		}
		if len(tasks) > 0 {
			m.taskPanel.tasks = tasks
			m.taskPanel.visible = true
		}
	default:
		return false
	}
	return true
}

// upsertTask replaces a task by ID, or appends if new.
func upsertTask(tasks []TaskEntry, task TaskEntry) []TaskEntry {
	for i := range tasks {
		if tasks[i].ID == task.ID {
			tasks[i] = task
			return tasks
		}
	}
	return append(tasks, task)
}

// Tasks returns the current task list (for testing).
func (m Model) Tasks() []TaskEntry {
	return m.taskPanel.tasks
}

// TasksVisible returns whether the task panel is visible (for testing).
func (m Model) TasksVisible() bool {
	return m.taskPanel.visible
}

// ToggleTasks toggles the task panel visibility.
func (m *Model) ToggleTasks() {
	m.taskPanel.visible = !m.taskPanel.visible
}

// HandleToolCallExported is an exported wrapper for handleToolCall (for testing).
func (m *Model) HandleToolCallExported(msg AgentToolCallMsg) {
	m.handleToolCall(msg)
}

// --- Task panel rendering ---

// taskCheckbox returns the checkbox character for a task status.
func taskCheckbox(status string) string {
	switch status {
	case "completed":
		return TranscriptMarkerDone.Render("✓")
	case "in_progress":
		return TranscriptMarkerRunning.Render("◼")
	default:
		return DimStyle.Render("◻")
	}
}

// taskDescriptionStyle returns styled description based on status and recency.
func taskDescriptionStyle(desc, status string, changedAt time.Time) string {
	recentlyChanged := time.Since(changedAt) < 2*time.Second
	if recentlyChanged && status == "completed" {
		return TranscriptMarkerDone.Render(desc)
	}
	if status == "completed" {
		return DimStyle.Render(desc)
	}
	if recentlyChanged {
		return AccentStyle.Render(desc)
	}
	return desc
}

// RenderTaskPanel renders the task checklist panel.
// Returns empty string if no tasks.
func RenderTaskPanel(tasks []TaskEntry, width int) string {
	if len(tasks) == 0 {
		return ""
	}

	var b strings.Builder

	// Progress counter
	completed := 0
	for _, t := range tasks {
		if t.Status == "completed" {
			completed++
		}
	}

	header := fmt.Sprintf("%d/%d tasks completed", completed, len(tasks))
	b.WriteString(transcriptIndent)
	b.WriteString(TranscriptConnectorStyle.Render(transcriptConnector))
	b.WriteString("  ")
	b.WriteString(DimStyle.Render(header))
	b.WriteString("\n")

	// Task list
	for _, task := range tasks {
		checkbox := taskCheckbox(task.Status)
		desc := taskDescriptionStyle(task.Description, task.Status, task.ChangedAt)

		b.WriteString(transcriptIndent)
		b.WriteString("   ")
		b.WriteString(checkbox)
		b.WriteString(" ")
		b.WriteString(desc)
		b.WriteString("\n")
	}

	return b.String()
}
