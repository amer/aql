package tui_test

import (
	"testing"

	"github.com/amer/aql/internal/domain"
	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskPanel_CreateAddsTask(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)

	// Simulate task_create done event
	m.HandleToolCallExported(tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall: domain.ToolCall{
			Name:    "task_create",
			Content: `{"id":1,"description":"write tests","status":"pending"}`,
			Status:  domain.ToolDone,
			ToolID:  "tool_1",
		},
	})

	tasks := m.Tasks()
	require.Len(t, tasks, 1)
	assert.Equal(t, 1, tasks[0].ID)
	assert.Equal(t, "write tests", tasks[0].Description)
	assert.Equal(t, "pending", tasks[0].Status)
}

func TestTaskPanel_UpdateChangesStatus(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)

	// Create a task
	m.HandleToolCallExported(tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall: domain.ToolCall{
			Name:    "task_create",
			Content: `{"id":1,"description":"write tests","status":"pending"}`,
			Status:  domain.ToolDone,
			ToolID:  "tool_1",
		},
	})

	// Update its status
	m.HandleToolCallExported(tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall: domain.ToolCall{
			Name:    "task_update",
			Content: `{"id":1,"description":"write tests","status":"completed"}`,
			Status:  domain.ToolDone,
			ToolID:  "tool_2",
		},
	})

	tasks := m.Tasks()
	require.Len(t, tasks, 1)
	assert.Equal(t, "completed", tasks[0].Status)
}

func TestTaskPanel_SuppressedFromTranscript(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)

	// Running phase — should also be suppressed
	m.HandleToolCallExported(tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall: domain.ToolCall{
			Name:    "task_create",
			Content: `{"description":"write tests"}`,
			Status:  domain.ToolRunning,
			ToolID:  "tool_1",
		},
	})

	// Done phase
	m.HandleToolCallExported(tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall: domain.ToolCall{
			Name:    "task_create",
			Content: `{"id":1,"description":"write tests","status":"pending"}`,
			Status:  domain.ToolDone,
			ToolID:  "tool_1",
		},
	})

	// Chat should have no tool entries for task tools
	for _, entry := range m.Chat() {
		if entry.Type == tui.EntryAgentTool && entry.ToolCall != nil {
			assert.NotContains(t, entry.ToolCall.Name, "task_",
				"task tools should not appear in transcript")
		}
	}
}

func TestTaskPanel_ToggleVisibility(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)

	// Should be visible by default when tasks exist
	m.HandleToolCallExported(tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall: domain.ToolCall{
			Name:    "task_create",
			Content: `{"id":1,"description":"task one","status":"pending"}`,
			Status:  domain.ToolDone,
			ToolID:  "tool_1",
		},
	})
	assert.True(t, m.TasksVisible())

	// Toggle off
	m.ToggleTasks()
	assert.False(t, m.TasksVisible())

	// Toggle back on
	m.ToggleTasks()
	assert.True(t, m.TasksVisible())
}

func TestTaskPanel_RenderCheckboxes(t *testing.T) {
	panel := tui.RenderTaskPanel([]tui.TaskEntry{
		{ID: 1, Description: "pending task", Status: "pending"},
		{ID: 2, Description: "active task", Status: "in_progress"},
		{ID: 3, Description: "done task", Status: "completed"},
	}, 80)

	assert.Contains(t, panel, "◻") // pending checkbox
	assert.Contains(t, panel, "◼") // in_progress checkbox
	assert.Contains(t, panel, "✓") // completed check
	assert.Contains(t, panel, "pending task")
	assert.Contains(t, panel, "active task")
	assert.Contains(t, panel, "done task")
}

func TestTaskPanel_ProgressCounter(t *testing.T) {
	panel := tui.RenderTaskPanel([]tui.TaskEntry{
		{ID: 1, Description: "done", Status: "completed"},
		{ID: 2, Description: "also done", Status: "completed"},
		{ID: 3, Description: "not done", Status: "pending"},
	}, 80)

	assert.Contains(t, panel, "2/3")
}

func TestTaskPanel_EmptyReturnsNothing(t *testing.T) {
	panel := tui.RenderTaskPanel(nil, 80)
	assert.Empty(t, panel)
}

func TestTaskPanel_ListParsesArray(t *testing.T) {
	m := tui.NewModel("test", []string{"coder"}, nil)

	m.HandleToolCallExported(tui.AgentToolCallMsg{
		AgentName: "coder",
		ToolCall: domain.ToolCall{
			Name:    "task_list",
			Content: `[{"id":1,"description":"A","status":"pending"},{"id":2,"description":"B","status":"completed"}]`,
			Status:  domain.ToolDone,
			ToolID:  "tool_1",
		},
	})

	tasks := m.Tasks()
	require.Len(t, tasks, 2)
	assert.Equal(t, "A", tasks[0].Description)
	assert.Equal(t, "B", tasks[1].Description)
}

func TestTaskDisplayNames(t *testing.T) {
	assert.Equal(t, "Create Task", tui.ToolDisplayName("task_create"))
	assert.Equal(t, "Update Task", tui.ToolDisplayName("task_update"))
	assert.Equal(t, "List Tasks", tui.ToolDisplayName("task_list"))
}
