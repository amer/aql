package main

import (
	"encoding/json"
	"testing"

	"github.com/amer/aql/internal/agent/tools"
	"github.com/stretchr/testify/assert"
)

func TestIsApproval(t *testing.T) {
	tests := []struct {
		answer string
		want   bool
	}{
		{"y", true},
		{"Y", true},
		{"yes", true},
		{"  YES  ", true},
		{"n", false},
		{"no", false},
		{"", false},
		{"sure", false},
		{"yeah", false},
	}
	for _, tt := range tests {
		t.Run(tt.answer, func(t *testing.T) {
			assert.Equal(t, tt.want, isApproval(tt.answer))
		})
	}
}

func TestApprovalPrompt(t *testing.T) {
	tests := []struct {
		name  string
		req   tools.ApprovalRequest
		wants string
	}{
		{
			name:  "bash shows command",
			req:   tools.ApprovalRequest{Tool: "bash", Input: json.RawMessage(`{"command":"rm -rf /"}`)},
			wants: "run: rm -rf /",
		},
		{
			name:  "edit shows file path",
			req:   tools.ApprovalRequest{Tool: "edit", Input: json.RawMessage(`{"file_path":"main.go"}`)},
			wants: "edit main.go",
		},
		{
			name:  "write_file shows path",
			req:   tools.ApprovalRequest{Tool: "write_file", Input: json.RawMessage(`{"path":"out.txt"}`)},
			wants: "write out.txt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := approvalPrompt(tt.req)
			assert.Contains(t, got, tt.wants)
			assert.Contains(t, got, tt.req.Tool)
			assert.Contains(t, got, "(y/n)")
		})
	}
}

func TestApprovalPrompt_UnknownShapeFallsBack(t *testing.T) {
	got := approvalPrompt(tools.ApprovalRequest{Tool: "bash", Input: json.RawMessage(`{}`)})
	assert.Equal(t, "Allow bash? (y/n)", got)
}
