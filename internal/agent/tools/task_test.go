package tools_test

import (
	"testing"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskStore_Create(t *testing.T) {
	store := tools.NewTaskStore()

	task := store.Create("implement feature X")

	assert.Equal(t, 1, task.ID)
	assert.Equal(t, "implement feature X", task.Description)
	assert.Equal(t, "pending", task.Status)
}

func TestTaskStore_Create_IncrementsID(t *testing.T) {
	store := tools.NewTaskStore()

	t1 := store.Create("first")
	t2 := store.Create("second")

	assert.Equal(t, 1, t1.ID)
	assert.Equal(t, 2, t2.ID)
}

func TestTaskStore_Update_ChangesStatus(t *testing.T) {
	store := tools.NewTaskStore()
	store.Create("task one")

	updated, err := store.Update(1, "in_progress")

	require.NoError(t, err)
	assert.Equal(t, "in_progress", updated.Status)
	assert.Equal(t, "task one", updated.Description)
}

func TestTaskStore_Update_Completed(t *testing.T) {
	store := tools.NewTaskStore()
	store.Create("task one")

	updated, err := store.Update(1, "completed")

	require.NoError(t, err)
	assert.Equal(t, "completed", updated.Status)
}

func TestTaskStore_Update_InvalidID(t *testing.T) {
	store := tools.NewTaskStore()

	_, err := store.Update(99, "in_progress")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "task 99 not found")
}

func TestTaskStore_Update_InvalidStatus(t *testing.T) {
	store := tools.NewTaskStore()
	store.Create("task one")

	_, err := store.Update(1, "invalid_status")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestTaskStore_List_Empty(t *testing.T) {
	store := tools.NewTaskStore()

	tasks := store.List()

	assert.Empty(t, tasks)
}

func TestTaskStore_List_ReturnsCopy(t *testing.T) {
	store := tools.NewTaskStore()
	store.Create("first")
	store.Create("second")

	tasks := store.List()

	assert.Len(t, tasks, 2)
	assert.Equal(t, "first", tasks[0].Description)
	assert.Equal(t, "second", tasks[1].Description)

	// Mutating the returned slice should not affect the store
	tasks[0].Description = "mutated"
	original := store.List()
	assert.Equal(t, "first", original[0].Description)
}
