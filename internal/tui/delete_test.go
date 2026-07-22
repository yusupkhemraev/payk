package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestDeleteRequestWithConfirmation(t *testing.T) {
	dir := fixtureWorkspace(t)
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	// G lands on ping; d opens the confirmation.
	m = typeString(m, "G")
	m = typeString(m, "d")
	view := plainView(m)
	if !strings.Contains(view, `delete "ping"? (y/n)`) {
		t.Fatalf("confirmation prompt missing:\n%s", view)
	}

	// n cancels: file stays.
	m = typeString(m, "n")
	if _, err := os.Stat(filepath.Join(dir, "collections", "api", "ping.yaml")); err != nil {
		t.Fatal("declined delete must keep the file:", err)
	}

	// y removes the file and the tree row.
	m = typeString(m, "d")
	m = typeString(m, "y")
	if _, err := os.Stat(filepath.Join(dir, "collections", "api", "ping.yaml")); !os.IsNotExist(err) {
		t.Errorf("file should be deleted, stat err = %v", err)
	}
	if strings.Contains(plainView(m), "ping") {
		t.Errorf("tree should not show the deleted request:\n%s", plainView(m))
	}
}

func TestDeleteCollectionRemovesDirectory(t *testing.T) {
	dir := fixtureWorkspace(t)
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	// Cursor starts on the collection row.
	m = typeString(m, "d")
	m = typeString(m, "y")

	if _, err := os.Stat(filepath.Join(dir, "collections", "api")); !os.IsNotExist(err) {
		t.Errorf("collection dir should be gone, stat err = %v", err)
	}
	if !strings.Contains(plainView(m), "no requests yet") {
		t.Errorf("tree should be empty:\n%s", plainView(m))
	}
}

func TestMessageLogShowsFullText(t *testing.T) {
	var m tea.Model = New(testConfig(t))
	m = stepMsg(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = runCmds(m, m.Init())

	longMsg := "import warning: body media type multipart/form-data imported without example " +
		strings.Repeat("x", 120)
	model := m.(Model)
	model.setStatus(longMsg, true)
	m = model

	// The status bar truncates.
	if strings.Contains(plainView(m), strings.Repeat("x", 120)) {
		t.Fatal("status bar should truncate long messages")
	}

	// The m overlay shows the full text, wrapped.
	m = typeString(m, "m")
	view := plainView(m)
	if !strings.Contains(view, "messages") {
		t.Fatalf("message log overlay missing:\n%s", view)
	}
	if !strings.Contains(collapseView(view), "multipart/form-data") ||
		!strings.Contains(collapseView(view), strings.Repeat("x", 60)) {
		t.Errorf("overlay should show the full message:\n%s", view)
	}

	// esc closes it.
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if strings.Contains(plainView(m), "m / esc to close") {
		t.Error("overlay should close on esc")
	}
}
