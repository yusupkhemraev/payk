package panels

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/tui/theme"
)

// Response is the right panel: the response viewer. Body/headers/timings
// tabs arrive in milestone 3 together with request sending.
type Response struct {
	theme   *theme.Theme
	width   int
	height  int
	focused bool
}

// NewResponse builds the response viewer panel.
func NewResponse(t *theme.Theme) Response {
	return Response{theme: t}
}

// SetSize sets the outer box size, borders included.
func (m *Response) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused toggles keyboard focus for this panel.
func (m *Response) SetFocused(focused bool) {
	m.focused = focused
}

// Update handles panel input; the viewer becomes interactive in milestone 3.
func (m Response) Update(_ tea.Msg) (Response, tea.Cmd) {
	return m, nil
}

// View renders the panel at its current size.
func (m Response) View() string {
	tabs := m.theme.Muted.Render("Body · Headers · Timings")

	body := strings.Join([]string{
		tabs,
		"",
		m.theme.Muted.Render(" no response yet — sending lands in milestone 3"),
	}, "\n")

	return frame(m.theme, "Response", m.focused, m.width, m.height, body)
}
