package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestCreateRequestInFolder(t *testing.T) {
	dir := fixtureWorkspace(t)
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	// Cursor on the users folder; a opens the new-request prompt.
	m = typeString(m, "j")
	m = typeString(m, "a")
	view := plainView(m)
	if !strings.Contains(view, "new request:") {
		t.Fatalf("new request prompt missing:\n%s", view)
	}

	m = typeString(m, "get profile")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	// The file lands inside the folder.
	if _, err := os.Stat(filepath.Join(dir, "collections", "api", "users", "get-profile.yaml")); err != nil {
		t.Fatalf("created file missing: %v", err)
	}

	// The editor opens the new request with focus.
	model := m.(Model)
	if model.focus != paneRequest {
		t.Errorf("focus = %v, want request editor", model.focus)
	}
	req := model.request.CurrentRequest()
	if req == nil || req.Name != "get profile" || req.Method != "GET" {
		t.Errorf("editor request = %+v", req)
	}
	if !strings.Contains(plainView(m), "created get profile") {
		t.Errorf("status should confirm creation:\n%s", plainView(m))
	}

	// :w after editing works against the created location.
	if model.selCollection != "api" || len(model.selPath) != 1 || model.selPath[0] != "users" {
		t.Errorf("selection = %q %v", model.selCollection, model.selPath)
	}
}

func TestCreateRequestInEmptyWorkspace(t *testing.T) {
	dir := t.TempDir()
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	m = typeString(m, "a")
	m = typeString(m, "first route")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if _, err := os.Stat(filepath.Join(dir, "collections", "api", "first-route.yaml")); err != nil {
		t.Fatalf("created file missing: %v", err)
	}
	view := plainView(m)
	if !strings.Contains(view, "first route") {
		t.Errorf("tree should show the new request:\n%s", view)
	}

	// esc cancels without creating.
	m = typeString(m, "h")
	m = typeString(m, "a")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if strings.Contains(plainView(m), "new request:") {
		t.Error("esc should close the prompt")
	}
}
