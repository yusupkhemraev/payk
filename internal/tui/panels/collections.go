package panels

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/tui/theme"
)

// demoEntry is placeholder tree content until real collections arrive with
// YAML storage in milestone 2.
type demoEntry struct {
	method string
	name   string
	folder bool
}

// Collections is the left panel: the collections/requests tree.
type Collections struct {
	theme   *theme.Theme
	width   int
	height  int
	focused bool

	entries  []demoEntry
	cursor   int
	pendingG bool
}

// NewCollections builds the collections panel.
func NewCollections(t *theme.Theme) Collections {
	return Collections{
		theme: t,
		entries: []demoEntry{
			{folder: true, name: "examples"},
			{method: "GET", name: "list users"},
			{method: "POST", name: "create user"},
			{method: "PUT", name: "replace user"},
			{method: "PATCH", name: "update user"},
			{method: "DELETE", name: "delete user"},
			{folder: true, name: "auth"},
			{method: "POST", name: "login"},
			{method: "GET", name: "me"},
		},
	}
}

// SetSize sets the outer box size, borders included.
func (m *Collections) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused toggles keyboard focus for this panel.
func (m *Collections) SetFocused(focused bool) {
	m.focused = focused
	if !focused {
		m.pendingG = false
	}
}

// Update handles navigation keys while the panel is focused.
func (m Collections) Update(msg tea.Msg) (Collections, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	// gg chord: first g arms the chord, second g jumps to top.
	if m.pendingG {
		m.pendingG = false
		if keyMsg.String() == "g" {
			m.cursor = 0
		}
		return m, nil
	}

	switch {
	case key.Matches(keyMsg, keyBinding("j", "down")):
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}
	case key.Matches(keyMsg, keyBinding("k", "up")):
		if m.cursor > 0 {
			m.cursor--
		}
	case keyMsg.String() == "g":
		m.pendingG = true
	case keyMsg.String() == "G":
		m.cursor = len(m.entries) - 1
	}
	return m, nil
}

func keyBinding(keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...))
}

// View renders the panel at its current size.
func (m Collections) View() string {
	var b strings.Builder
	for i, e := range m.entries {
		line := m.renderEntry(e)
		if i == m.cursor && m.focused {
			line = m.theme.Selected.Render(" " + line + " ")
		} else {
			line = " " + line
		}
		b.WriteString(line)
		if i < len(m.entries)-1 {
			b.WriteString("\n")
		}
	}
	return frame(m.theme, "Collections", m.focused, m.width, m.height, b.String())
}

func (m Collections) renderEntry(e demoEntry) string {
	if e.folder {
		return m.theme.Muted.Render("▾ " + e.name)
	}
	badge := m.theme.Method(e.method).Render(fmt.Sprintf("%-6s", e.method))
	return "  " + badge + " " + e.name
}
