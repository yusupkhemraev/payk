package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// The selected row is rendered from plain text, so it has to pick up the
// configured glyphs like every other row.
func TestSelectedRowUsesConfiguredIcons(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := fixtureWorkspace(t)

	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m = runCmds(m, m.Init())
	m = runCommand(m, "icons nerd")

	// Row 1 is the collapsed users folder: select it and check the glyph.
	m = typeString(m, "j")
	view := plainView(m)
	if strings.Contains(view, "▸ users") || strings.Contains(view, "▾ users") {
		t.Errorf("selected folder still shows the unicode arrow:\n%s", view)
	}
	if !strings.Contains(view, " users") {
		t.Errorf("selected folder should use the nerd glyph:\n%s", view)
	}
}
