package tools_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutor_ApproverDeniesBashDoesNotExecute(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "sentinel")

	exec := tools.NewExecutor(tools.WithApprover(
		func(context.Context, tools.ApprovalRequest) (bool, error) { return false, nil },
	))

	out, err := exec(context.Background(), dir, "bash",
		json.RawMessage(`{"command":"touch `+sentinel+`"}`))
	require.NoError(t, err)
	assert.Contains(t, out, "denied")
	assert.NoFileExists(t, sentinel, "denied bash command must not run")
}

func TestExecutor_ApproverAllowsBashExecutes(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "sentinel")

	var gotReq tools.ApprovalRequest
	exec := tools.NewExecutor(tools.WithApprover(
		func(_ context.Context, req tools.ApprovalRequest) (bool, error) {
			gotReq = req
			return true, nil
		},
	))

	_, err := exec(context.Background(), dir, "bash",
		json.RawMessage(`{"command":"touch `+sentinel+`"}`))
	require.NoError(t, err)
	assert.FileExists(t, sentinel, "approved bash command must run")
	assert.Equal(t, "bash", gotReq.Tool)
	assert.Equal(t, dir, gotReq.WorkDir)
}

func TestExecutor_ApproverNotCalledForReadOnlyTools(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "test.go", "package main")

	called := false
	exec := tools.NewExecutor(tools.WithApprover(
		func(context.Context, tools.ApprovalRequest) (bool, error) {
			called = true
			return true, nil
		},
	))

	_, err := exec(context.Background(), dir, "read_file", json.RawMessage(`{"path":"test.go"}`))
	require.NoError(t, err)
	assert.False(t, called, "read-only tools must not require approval")
}

func TestExecutor_NoApproverExecutesWithoutGate(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "sentinel")

	exec := tools.NewExecutor()
	_, err := exec(context.Background(), dir, "bash",
		json.RawMessage(`{"command":"touch `+sentinel+`"}`))
	require.NoError(t, err)
	assert.FileExists(t, sentinel, "with no approver configured, tools run unchanged")
}
