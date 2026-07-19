package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func helpAt(t *testing.T, width, height int) (tea.Model, string) {
	t.Helper()
	var m tea.Model = New(testConfig(t))
	m = stepMsg(m, tea.WindowSizeMsg{Width: width, Height: height})
	m = runCmds(m, m.Init())
	m = typeString(m, "?")
	return m, ansi.Strip(m.(Model).View().Content)
}

func TestHelpAdaptsToTerminalSize(t *testing.T) {
	sizes := [][2]int{{160, 45}, {120, 40}, {100, 30}, {80, 24}, {60, 20}}
	for _, size := range sizes {
		width, height := size[0], size[1]
		_, view := helpAt(t, width, height)

		lines := strings.Split(view, "\n")
		if len(lines) != height {
			t.Errorf("%dx%d: rendered %d lines, want %d", width, height, len(lines), height)
		}
		for i, line := range lines {
			if w := ansi.StringWidth(line); w > width {
				t.Errorf("%dx%d: line %d is %d cells wide, max %d", width, height, i, w, width)
			}
		}
		if !strings.Contains(view, "keybindings") {
			t.Errorf("%dx%d: title missing", width, height)
		}
	}
}

func TestHelpScrollsWhenTooTall(t *testing.T) {
	m, view := helpAt(t, 60, 20)

	// The small terminal cannot fit every binding: a scroll footer appears
	// and the last binding (quit) is off screen.
	if !strings.Contains(view, "j/k scroll") {
		t.Fatalf("scroll footer missing on a small terminal:\n%s", view)
	}
	if strings.Contains(view, "message log") {
		t.Fatalf("late bindings should start off screen:\n%s", view)
	}

	// Scroll to the bottom: quit becomes visible.
	for range 40 {
		m = typeString(m, "j")
	}
	view = ansi.Strip(m.(Model).View().Content)
	if !strings.Contains(view, "message log") {
		t.Errorf("scrolling should reveal the late bindings:\n%s", view)
	}

	// Wide terminals fit everything: no footer.
	_, wide := helpAt(t, 160, 45)
	if strings.Contains(wide, "j/k scroll") {
		t.Errorf("no scroll footer expected on a large terminal:\n%s", wide)
	}
}
