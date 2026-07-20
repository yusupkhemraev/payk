package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestResizeFocusedPane(t *testing.T) {
	var m tea.Model = New(testConfig(t))
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	base := m.(Model).sizes

	// Widen the focused collections sidebar.
	m = typeString(m, ">>")
	sizes := m.(Model).sizes
	if sizes.Collections.Width != base.Collections.Width+4 {
		t.Errorf("sidebar width = %d, want %d", sizes.Collections.Width, base.Collections.Width+4)
	}
	if sizes.Collections.Width+sizes.Request.Width+sizes.Response.Width != 140 {
		t.Error("pane widths must still sum to the terminal width")
	}

	// Narrow it back.
	m = typeString(m, "<<")
	if got := m.(Model).sizes.Collections.Width; got != base.Collections.Width {
		t.Errorf("sidebar width = %d after shrink, want %d", got, base.Collections.Width)
	}

	// Widening the response pane moves the request/response split.
	m = typeString(m, "ll")
	m = typeString(m, ">>")
	sizes = m.(Model).sizes
	if sizes.Response.Width != base.Response.Width+4 {
		t.Errorf("response width = %d, want %d", sizes.Response.Width, base.Response.Width+4)
	}
	if sizes.Request.Width != base.Request.Width-4 {
		t.Errorf("request width = %d, want %d", sizes.Request.Width, base.Request.Width-4)
	}

	// Clamping: shrinking far past the minimum keeps panes usable.
	for range 40 {
		m = typeString(m, "<")
	}
	sizes = m.(Model).sizes
	if sizes.Response.Width < minPaneWidth {
		t.Errorf("response width = %d, must stay >= %d", sizes.Response.Width, minPaneWidth)
	}
	if sizes.Collections.Width+sizes.Request.Width+sizes.Response.Width != 140 {
		t.Error("pane widths must still sum to the terminal width after clamping")
	}
}
