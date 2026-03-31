package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func execNotebookEdit(workDir string, input json.RawMessage) (string, error) {
	var params struct {
		Path     string `json:"path"`
		CellIdx  int    `json:"cell_index"`
		NewSrc   string `json:"new_source"`
		CellType string `json:"cell_type"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "invalid input: " + err.Error(), nil
	}

	path := resolvePath(workDir, params.Path)
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error(), nil
	}

	var nb map[string]any
	if err := json.Unmarshal(data, &nb); err != nil {
		return fmt.Sprintf("parse notebook: %s", err), nil
	}

	cellsRaw, ok := nb["cells"]
	if !ok {
		return "notebook has no cells array", nil
	}
	cells, ok := cellsRaw.([]any)
	if !ok {
		return "notebook cells is not an array", nil
	}

	if params.CellIdx < 0 || params.CellIdx >= len(cells) {
		return fmt.Sprintf("cell_index %d out of range (notebook has %d cells)", params.CellIdx, len(cells)), nil
	}

	cell, ok := cells[params.CellIdx].(map[string]any)
	if !ok {
		return fmt.Sprintf("cell %d is not a JSON object", params.CellIdx), nil
	}

	// Split source into lines array (ipynb format)
	lines := splitSourceLines(params.NewSrc)
	cell["source"] = lines

	if params.CellType != "" {
		cell["cell_type"] = params.CellType
	}

	out, err := json.MarshalIndent(nb, "", "  ")
	if err != nil {
		return fmt.Sprintf("marshal notebook: %s", err), nil
	}
	// Append trailing newline for consistency
	out = append(out, '\n')

	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Sprintf("write notebook: %s", err), nil
	}

	return fmt.Sprintf("updated cell %d in %s", params.CellIdx, params.Path), nil
}

// splitSourceLines splits source text into the line array format used by .ipynb.
// Each line except the last gets a trailing newline.
func splitSourceLines(src string) []string {
	if src == "" {
		return []string{}
	}
	lines := strings.Split(src, "\n")
	result := make([]string, len(lines))
	for i, line := range lines {
		if i < len(lines)-1 {
			result[i] = line + "\n"
		} else {
			result[i] = line
		}
	}
	return result
}
