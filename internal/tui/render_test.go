package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// renderAt returns the plain-text render of the root model at a terminal size.
func renderAt(t *testing.T, width, height int, msgs ...tea.Msg) string {
	t.Helper()
	var m tea.Model = New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}
	return ansi.Strip(m.(Model).View().Content)
}

func TestRenderFillsTerminalExactly(t *testing.T) {
	sizes := [][2]int{{140, 24}, {120, 40}, {100, 20}, {80, 24}, {60, 20}}
	for _, size := range sizes {
		width, height := size[0], size[1]
		content := renderAt(t, width, height)
		lines := strings.Split(content, "\n")

		if len(lines) != height {
			t.Errorf("%dx%d: rendered %d lines, want %d", width, height, len(lines), height)
		}
		for i, line := range lines {
			if w := ansi.StringWidth(line); w > width {
				t.Errorf("%dx%d: line %d is %d cells wide, max %d", width, height, i, w, width)
			}
		}
	}
}

func TestRenderHelpOverlayFits(t *testing.T) {
	content := renderAt(t, 100, 30, tea.KeyPressMsg{Code: '?', Text: "?"})
	if !strings.Contains(content, "keybindings") {
		t.Fatal("help overlay not shown after pressing ?")
	}
	lines := strings.Split(content, "\n")
	if len(lines) != 30 {
		t.Errorf("help overlay: rendered %d lines, want 30", len(lines))
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w > 100 {
			t.Errorf("help overlay: line %d is %d cells wide, max 100", i, w)
		}
	}
}
