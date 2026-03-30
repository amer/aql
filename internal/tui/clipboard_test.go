package tui_test

import (
	"bytes"
	"encoding/base64"
	"os/exec"
	"testing"

	"github.com/amer/aql/internal/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOSC52Copy(t *testing.T) {
	var buf bytes.Buffer
	tui.OSC52Copy(&buf, "hello world")

	encoded := base64.StdEncoding.EncodeToString([]byte("hello world"))
	expected := "\x1b]52;c;" + encoded + "\x07"
	assert.Equal(t, expected, buf.String())
}

func TestOSC52CopyEmpty(t *testing.T) {
	var buf bytes.Buffer
	tui.OSC52Copy(&buf, "")

	encoded := base64.StdEncoding.EncodeToString([]byte(""))
	expected := "\x1b]52;c;" + encoded + "\x07"
	assert.Equal(t, expected, buf.String())
}

func TestOSC52CopyMultiLine(t *testing.T) {
	var buf bytes.Buffer
	tui.OSC52Copy(&buf, "line one\nline two\nline three")

	// Verify it starts with OSC 52 and ends with BEL
	assert.True(t, len(buf.String()) > 0)
	assert.Equal(t, byte('\x1b'), buf.Bytes()[0])
	assert.Equal(t, byte('\x07'), buf.Bytes()[buf.Len()-1])
}

func TestExecCopy(t *testing.T) {
	// Skip if pbcopy is not available (non-macOS)
	if _, err := exec.LookPath("pbcopy"); err != nil {
		t.Skip("pbcopy not available")
	}

	err := tui.ExecCopy("test clipboard text")
	require.NoError(t, err)

	// Verify by reading back with pbpaste
	out, err := exec.Command("pbpaste").Output()
	require.NoError(t, err)
	assert.Equal(t, "test clipboard text", string(out))
}

func TestExecCopyMultiLine(t *testing.T) {
	if _, err := exec.LookPath("pbcopy"); err != nil {
		t.Skip("pbcopy not available")
	}

	err := tui.ExecCopy("line one\nline two")
	require.NoError(t, err)

	out, err := exec.Command("pbpaste").Output()
	require.NoError(t, err)
	assert.Equal(t, "line one\nline two", string(out))
}

func TestExecCopyEmpty(t *testing.T) {
	if _, err := exec.LookPath("pbcopy"); err != nil {
		t.Skip("pbcopy not available")
	}

	err := tui.ExecCopy("")
	require.NoError(t, err)
}
