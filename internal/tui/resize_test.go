package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// workspaceWithConfig returns an empty workspace carrying a config.yaml.
func workspaceWithConfig(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestResizeColumnsLayout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := Config{WorkspaceDir: workspaceWithConfig(t, "layout: columns\n")}

	var m tea.Model = New(cfg)
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	base := m.(Model).sizes
	if base.Mode != ModeTriple {
		t.Fatalf("config should select the columns layout, got mode %v", base.Mode)
	}

	// Widen the focused collections sidebar.
	m = typeString(m, ">>")
	sizes := m.(Model).sizes
	if sizes.Collections.Width != base.Collections.Width+4 {
		t.Errorf("sidebar width = %d, want %d", sizes.Collections.Width, base.Collections.Width+4)
	}
	if sizes.Collections.Width+sizes.Request.Width+sizes.Response.Width+2*separatorSize != 140 {
		t.Error("panes plus rules must still cover the terminal width")
	}

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

	// Clamping: shrinking far past the minimum keeps panes usable.
	for range 40 {
		m = typeString(m, "<")
	}
	sizes = m.(Model).sizes
	if sizes.Response.Width < minPaneWidth {
		t.Errorf("response width = %d, must stay >= %d", sizes.Response.Width, minPaneWidth)
	}
	if sizes.Collections.Width+sizes.Request.Width+sizes.Response.Width+2*separatorSize != 140 {
		t.Error("panes plus rules must still cover the terminal width after clamping")
	}
}

func TestResizeStackedLayout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var m tea.Model = New(testConfig(t))
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	base := m.(Model).sizes
	if base.Mode != ModeStacked {
		t.Fatalf("stacked is the default layout, got mode %v", base.Mode)
	}
	if base.Request.Width != base.Response.Width {
		t.Errorf("stacked blocks share one width: %d vs %d",
			base.Request.Width, base.Response.Width)
	}
	if base.Collections.Width+base.Request.Width+separatorSize != 140 {
		t.Error("sidebar, rule, and stack must fill the terminal width")
	}

	// Focused on the request block, > grows it and shrinks the response.
	m = typeString(m, "l")
	m = typeString(m, ">>")
	sizes := m.(Model).sizes
	if sizes.Request.Height != base.Request.Height+4 {
		t.Errorf("request height = %d, want %d", sizes.Request.Height, base.Request.Height+4)
	}
	if sizes.Request.Height+sizes.Response.Height != base.Request.Height+base.Response.Height {
		t.Error("block heights must still sum to the pane area")
	}
	if sizes.Request.Width != sizes.Response.Width {
		t.Error("stacked blocks keep a shared width while resizing")
	}

	// Clamping keeps both blocks usable.
	for range 40 {
		m = typeString(m, ">")
	}
	sizes = m.(Model).sizes
	if sizes.Response.Height < minPaneHeight {
		t.Errorf("response height = %d, must stay >= %d", sizes.Response.Height, minPaneHeight)
	}
}
