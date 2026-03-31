package tools

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Task struct, TaskStore (thread-safe with mutex)
//   - Create/Update/List operations
//   - registerTaskTools — adds task handlers to registry
//   - execTaskCreate/execTaskUpdate/execTaskList
//   - validStatuses map
//
// MUST NOT GO HERE:
//   - TUI task panel rendering (tui/tasks.go)
//   - Tool definitions (defs.go)
//   - Anything that imports agent or tui
//
// Q: Should I add a new task field?
// A: Add it to Task struct here and update the TUI's TaskEntry in
//    tui/tasks.go.
//
// Q: Is TaskStore safe for concurrent use?
// A: Yes, all methods hold the mutex.
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Task represents a tracked unit of work.
type Task struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

var validStatuses = map[string]bool{
	"pending":     true,
	"in_progress": true,
	"completed":   true,
}

// TaskStore holds tasks for a single agent session.
// It is safe for concurrent use.
type TaskStore struct {
	mu     sync.Mutex
	tasks  []Task
	nextID int
}

// NewTaskStore creates an empty task store.
func NewTaskStore() *TaskStore {
	return &TaskStore{nextID: 1}
}

// Create adds a new task with status "pending" and returns it.
func (s *TaskStore) Create(description string) Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := Task{
		ID:          s.nextID,
		Description: description,
		Status:      "pending",
	}
	s.nextID++
	s.tasks = append(s.tasks, t)
	return t
}

// Update changes the status of a task by ID and returns the updated task.
func (s *TaskStore) Update(id int, status string) (Task, error) {
	if !validStatuses[status] {
		return Task{}, fmt.Errorf("invalid status %q: must be pending, in_progress, or completed", status)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.tasks {
		if s.tasks[i].ID == id {
			s.tasks[i].Status = status
			return s.tasks[i], nil
		}
	}
	return Task{}, fmt.Errorf("task %d not found", id)
}

// List returns a copy of all tasks.
func (s *TaskStore) List() []Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Task, len(s.tasks))
	copy(out, s.tasks)
	return out
}

// registerTaskTools adds task_create, task_update, task_list handlers to the registry.
func registerTaskTools(registry map[string]toolHandler, store *TaskStore) {
	registry["task_create"] = func(_ context.Context, _ string, input json.RawMessage) (string, error) {
		return execTaskCreate(input, store)
	}
	registry["task_update"] = func(_ context.Context, _ string, input json.RawMessage) (string, error) {
		return execTaskUpdate(input, store)
	}
	registry["task_list"] = func(_ context.Context, _ string, _ json.RawMessage) (string, error) {
		return execTaskList(store)
	}
}

func execTaskCreate(input json.RawMessage, store *TaskStore) (string, error) {
	var params struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "invalid input: " + err.Error(), nil
	}
	if params.Description == "" {
		return "description is required", nil
	}
	task := store.Create(params.Description)
	out, _ := json.Marshal(task)
	return string(out), nil
}

func execTaskUpdate(input json.RawMessage, store *TaskStore) (string, error) {
	var params struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "invalid input: " + err.Error(), nil
	}
	task, err := store.Update(params.ID, params.Status)
	if err != nil {
		return err.Error(), nil
	}
	out, _ := json.Marshal(task)
	return string(out), nil
}

func execTaskList(store *TaskStore) (string, error) {
	tasks := store.List()
	out, _ := json.Marshal(tasks)
	return string(out), nil
}
