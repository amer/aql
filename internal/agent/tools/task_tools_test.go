package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskCreateTool(t *testing.T) {
	store := tools.NewTaskStore()
	exec := tools.NewExecutor(tools.WithTaskStore(store))

	result, err := exec(context.Background(), "/tmp", "task_create", json.RawMessage(`{"description":"write tests"}`))

	require.NoError(t, err)

	var task tools.Task
	require.NoError(t, json.Unmarshal([]byte(result), &task))
	assert.Equal(t, 1, task.ID)
	assert.Equal(t, "write tests", task.Description)
	assert.Equal(t, "pending", task.Status)
}

func TestTaskCreateTool_MissingDescription(t *testing.T) {
	store := tools.NewTaskStore()
	exec := tools.NewExecutor(tools.WithTaskStore(store))

	result, err := exec(context.Background(), "/tmp", "task_create", json.RawMessage(`{}`))

	require.NoError(t, err)
	assert.Contains(t, result, "description is required")
}

func TestTaskUpdateTool(t *testing.T) {
	store := tools.NewTaskStore()
	store.Create("first task")
	exec := tools.NewExecutor(tools.WithTaskStore(store))

	result, err := exec(context.Background(), "/tmp", "task_update", json.RawMessage(`{"id":1,"status":"in_progress"}`))

	require.NoError(t, err)

	var task tools.Task
	require.NoError(t, json.Unmarshal([]byte(result), &task))
	assert.Equal(t, "in_progress", task.Status)
}

func TestTaskUpdateTool_InvalidID(t *testing.T) {
	store := tools.NewTaskStore()
	exec := tools.NewExecutor(tools.WithTaskStore(store))

	result, err := exec(context.Background(), "/tmp", "task_update", json.RawMessage(`{"id":99,"status":"completed"}`))

	require.NoError(t, err)
	assert.Contains(t, result, "task 99 not found")
}

func TestTaskListTool_Empty(t *testing.T) {
	store := tools.NewTaskStore()
	exec := tools.NewExecutor(tools.WithTaskStore(store))

	result, err := exec(context.Background(), "/tmp", "task_list", json.RawMessage(`{}`))

	require.NoError(t, err)
	assert.Equal(t, "[]", result)
}

func TestTaskListTool_WithTasks(t *testing.T) {
	store := tools.NewTaskStore()
	store.Create("task A")
	store.Create("task B")
	exec := tools.NewExecutor(tools.WithTaskStore(store))

	result, err := exec(context.Background(), "/tmp", "task_list", json.RawMessage(`{}`))

	require.NoError(t, err)

	var tasks []tools.Task
	require.NoError(t, json.Unmarshal([]byte(result), &tasks))
	assert.Len(t, tasks, 2)
	assert.Equal(t, "task A", tasks[0].Description)
}

func TestTaskToolsInDefinitions(t *testing.T) {
	defs := tools.Definitions()
	names := make(map[string]bool)
	for _, d := range defs {
		names[d.Name] = true
	}

	assert.True(t, names["task_create"], "task_create should be in Definitions()")
	assert.True(t, names["task_update"], "task_update should be in Definitions()")
	assert.True(t, names["task_list"], "task_list should be in Definitions()")
}
