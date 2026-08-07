package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// jumpWorkspace has enough requests that jumping beats pressing j.
func jumpWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{
		"list users", "create user", "delete user",
		"send otp", "check otp", "upload avatar",
	} {
		path := filepath.Join(dir, "collections", "api", strings.ReplaceAll(name, " ", "-")+".yaml")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "name: " + name + "\nmethod: GET\nurl: https://example.com/" + name + "\n"
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestJumpModeMovesCursorByLabel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var m tea.Model = New(Config{WorkspaceDir: jumpWorkspace(t)})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m = runCmds(m, m.Init())

	// g shows the labels without moving anything yet.
	m = typeString(m, "g")
	view := plainView(m)
	if !strings.Contains(view, "a ") || !strings.Contains(view, "s ") {
		t.Fatalf("jump labels missing:\n%s", view)
	}

	// The label on the fourth row jumps straight there.
	m = typeString(m, "f")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	model := m.(Model)
	req := model.request.CurrentRequest()
	if req == nil {
		t.Fatal("jump should land on a request that enter can open")
	}
	if req.Name == "list users" {
		t.Errorf("jump did not move the cursor, still on %q", req.Name)
	}
	if strings.Contains(plainView(m), "a  ▾ api") {
		t.Error("labels should disappear once the jump is done")
	}
}

func TestGGStillGoesToTop(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var m tea.Model = New(Config{WorkspaceDir: jumpWorkspace(t)})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m = runCmds(m, m.Init())

	m = typeString(m, "G")
	m = typeString(m, "gg")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	// Row 0 is the collection: enter toggles it instead of opening a request.
	if !strings.Contains(plainView(m), "▸ api") {
		t.Errorf("gg should return to the collection row:\n%s", plainView(m))
	}
}

func TestFuzzySearchRanksBestMatchFirst(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var m tea.Model = New(Config{WorkspaceDir: jumpWorkspace(t)})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m = runCmds(m, m.Init())

	// "cu" is not a substring of "create user" — only fuzzy matching finds
	// it, and it must outrank the other requests.
	m = typeString(m, "/")
	m = typeString(m, "cu")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	model := m.(Model)
	req := model.request.CurrentRequest()
	if req == nil || req.Name != "create user" {
		t.Errorf("fuzzy search should open create user, got %+v", req)
	}
}
