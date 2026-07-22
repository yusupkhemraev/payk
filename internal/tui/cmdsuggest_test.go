package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestCmdlineCompletion(t *testing.T) {
	dir := envWorkspace(t, "http://localhost:1")
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 160, Height: 30})
	m = runCmds(m, m.Init())

	// Opening : lists the commands.
	m = typeString(m, ":")
	view := plainView(m)
	for _, want := range []string{"send", "env", "import", "reimport", "tab complete"} {
		if !strings.Contains(view, want) {
			t.Fatalf("command list missing %q:\n%s", want, view)
		}
	}

	// Typing narrows: "se" leaves send/set; ctrl+n moves the highlight,
	// tab accepts the selected command.
	m = typeString(m, "se")
	view = plainView(m)
	if strings.Contains(view, "reimport") {
		t.Fatalf("non-matching commands should be filtered:\n%s", view)
	}
	m = stepMsg(m, tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	model := m.(Model)
	if got := model.cmdline.value(); got != "set " {
		t.Fatalf("tab should accept the highlighted command, got %q", got)
	}

	// After "set " the active environment's variables are suggested.
	view = plainView(m)
	if !strings.Contains(view, "base_url") {
		t.Fatalf("variable suggestions missing after set:\n%s", view)
	}
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	model = m.(Model)
	if got := model.cmdline.value(); got != "set base_url " {
		t.Fatalf("tab should complete the variable, got %q", got)
	}
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	// env completion offers environment names and new.
	m = typeString(m, ":")
	m = stepMsg(m, tea.PasteMsg{Content: "env "})
	view = plainView(m)
	for _, want := range []string{"local", "staging", "new"} {
		if !strings.Contains(view, want) {
			t.Errorf("env completion missing %q:\n%s", want, view)
		}
	}
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	// reimport completion offers collections with recorded sources.
	spec := filepath.Join(dir, "openapi.json")
	if err := os.WriteFile(spec, []byte(specV1), 0o644); err != nil {
		t.Fatal(err)
	}
	m = runCommand(m, "import "+spec)
	m = typeString(m, ":")
	m = stepMsg(m, tea.PasteMsg{Content: "reimport "})
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	model = m.(Model)
	if got := model.cmdline.value(); got != "reimport shop" {
		t.Errorf("reimport completion should offer the collection, got %q", got)
	}
}
