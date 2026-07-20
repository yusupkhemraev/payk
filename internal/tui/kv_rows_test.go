package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestAddMultipleHeaderRows(t *testing.T) {
	cfg := Config{WorkspaceDir: fixtureWorkspace(t)}
	var m tea.Model = New(cfg)
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	// Open ping, focus the editor, go to the Headers tab.
	m = typeString(m, "G")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = typeString(m, "l")
	m = typeString(m, "]]")

	// First header.
	m = typeString(m, "a")
	m = typeString(m, "X-One")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	m = typeString(m, "1")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	// Second header.
	m = typeString(m, "a")
	m = typeString(m, "X-Two")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	m = typeString(m, "2")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	// Third header committed with enter: it chains straight into a fresh
	// row, so the next header can be typed without pressing a again.
	m = typeString(m, "a")
	m = typeString(m, "X-Three")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	m = typeString(m, "3")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	model := m.(Model)
	if !model.request.Editing() {
		t.Fatal("enter on a filled row should chain into a new row in insert mode")
	}

	// esc drops the trailing empty row.
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	model = m.(Model)
	req := model.request.CurrentRequest()
	if len(req.Headers) != 3 {
		t.Fatalf("headers = %+v, want 3 rows", req.Headers)
	}
	for i, want := range []struct{ name, value string }{
		{"X-One", "1"}, {"X-Two", "2"}, {"X-Three", "3"},
	} {
		if req.Headers[i].Name != want.name || req.Headers[i].Value != want.value {
			t.Errorf("header %d = %+v, want %s=%s", i, req.Headers[i], want.name, want.value)
		}
	}

	view := plainView(m)
	for _, want := range []string{"X-One = 1", "X-Two = 2", "X-Three = 3"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}
