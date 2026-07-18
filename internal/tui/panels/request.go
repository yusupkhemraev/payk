package panels

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/tui/theme"
)

// Request is the center panel: the request editor. Editing tabs (params,
// headers, body, auth) arrive in milestone 3; for now it renders a static
// preview so layout and focus behavior can be exercised.
type Request struct {
	theme   *theme.Theme
	width   int
	height  int
	focused bool
}

// NewRequest builds the request editor panel.
func NewRequest(t *theme.Theme) Request {
	return Request{theme: t}
}

// SetSize sets the outer box size, borders included.
func (m *Request) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused toggles keyboard focus for this panel.
func (m *Request) SetFocused(focused bool) {
	m.focused = focused
}

// Update handles panel input; the editor becomes interactive in milestone 3.
func (m Request) Update(_ tea.Msg) (Request, tea.Cmd) {
	return m, nil
}

// View renders the panel at its current size.
func (m Request) View() string {
	method := m.theme.Method("GET").Render("GET")
	tabs := m.theme.Muted.Render("URL · Params · Headers · Body · Auth")

	body := strings.Join([]string{
		tabs,
		"",
		" " + method + " https://api.example.com/users",
		"",
		m.theme.Muted.Render(" request editing lands in milestone 3"),
	}, "\n")

	return frame(m.theme, "Request", m.focused, m.width, m.height, body)
}
