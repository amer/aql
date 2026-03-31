package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleNotebook = `{
  "cells": [
    {
      "cell_type": "code",
      "source": ["print('hello')"],
      "metadata": {},
      "outputs": [],
      "execution_count": null
    },
    {
      "cell_type": "markdown",
      "source": ["# Title"],
      "metadata": {}
    }
  ],
  "metadata": {
    "kernelspec": {
      "display_name": "Python 3",
      "language": "python",
      "name": "python3"
    }
  },
  "nbformat": 4,
  "nbformat_minor": 5
}`

func writeNotebook(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

func TestNotebookEdit_ReplacesCellSource(t *testing.T) {
	dir := t.TempDir()
	nbPath := writeNotebook(t, dir, "test.ipynb", sampleNotebook)
	exec := tools.NewExecutor()

	result, err := exec(context.Background(), dir, "notebook_edit",
		json.RawMessage(`{"path":"test.ipynb","cell_index":0,"new_source":"print('world')"}`))

	require.NoError(t, err)
	assert.Contains(t, result, "updated cell 0")

	// Verify the file was actually modified
	data, readErr := os.ReadFile(nbPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "print('world')")
	assert.NotContains(t, string(data), "print('hello')")
}

func TestNotebookEdit_ChangesCellType(t *testing.T) {
	dir := t.TempDir()
	writeNotebook(t, dir, "test.ipynb", sampleNotebook)
	exec := tools.NewExecutor()

	result, err := exec(context.Background(), dir, "notebook_edit",
		json.RawMessage(`{"path":"test.ipynb","cell_index":0,"new_source":"# Now markdown","cell_type":"markdown"}`))

	require.NoError(t, err)
	assert.Contains(t, result, "updated cell 0")

	data, _ := os.ReadFile(filepath.Join(dir, "test.ipynb"))
	var nb map[string]any
	json.Unmarshal(data, &nb)
	cells := nb["cells"].([]any)
	cell := cells[0].(map[string]any)
	assert.Equal(t, "markdown", cell["cell_type"])
}

func TestNotebookEdit_OutOfBoundsIndex(t *testing.T) {
	dir := t.TempDir()
	writeNotebook(t, dir, "test.ipynb", sampleNotebook)
	exec := tools.NewExecutor()

	result, err := exec(context.Background(), dir, "notebook_edit",
		json.RawMessage(`{"path":"test.ipynb","cell_index":99,"new_source":"x"}`))

	require.NoError(t, err)
	assert.Contains(t, result, "cell_index 99 out of range")
}

func TestNotebookEdit_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	exec := tools.NewExecutor()

	result, err := exec(context.Background(), dir, "notebook_edit",
		json.RawMessage(`{"path":"missing.ipynb","cell_index":0,"new_source":"x"}`))

	require.NoError(t, err)
	assert.Contains(t, result, "no such file")
}

func TestNotebookEdit_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.ipynb"), []byte("not json"), 0644)
	exec := tools.NewExecutor()

	result, err := exec(context.Background(), dir, "notebook_edit",
		json.RawMessage(`{"path":"bad.ipynb","cell_index":0,"new_source":"x"}`))

	require.NoError(t, err)
	assert.Contains(t, result, "parse notebook")
}

func TestNotebookEditInDefinitions(t *testing.T) {
	defs := tools.Definitions()
	found := false
	for _, d := range defs {
		if d.Name == "notebook_edit" {
			found = true
			break
		}
	}
	assert.True(t, found, "notebook_edit should be in Definitions()")
}
