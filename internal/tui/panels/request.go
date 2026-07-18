package panels

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/core"
	"github.com/yusupkhemraev/payk/internal/tui/theme"
)

// Request is the center panel: the request editor. Until editing lands in
// milestone 3 it renders a read-only preview of the selected request.
type Request struct {
	theme   *theme.Theme
	width   int
	height  int
	focused bool

	req *core.Request
}

// NewRequest builds the request editor panel.
func NewRequest(t *theme.Theme) Request {
	return Request{theme: t}
}

// SetRequest replaces the request shown in the editor.
func (m *Request) SetRequest(req *core.Request) {
	m.req = req
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
	return frame(m.theme, "Request", m.focused, m.width, m.height, m.body())
}

func (m Request) body() string {
	tabs := m.theme.Muted.Render("URL · Params · Headers · Body · Auth")
	if m.req == nil {
		return tabs + "\n\n" + m.theme.Muted.Render(" select a request in collections (enter)")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", tabs)
	fmt.Fprintf(&b, " %s %s\n", m.theme.Method(m.req.Method).Render(m.req.Method), m.req.URL)

	if len(m.req.Params) > 0 {
		fmt.Fprintf(&b, "\n%s\n", m.theme.Muted.Render(" params"))
		for _, p := range m.req.Params {
			fmt.Fprintf(&b, "   %s = %s\n", p.Name, p.Value)
		}
	}
	if len(m.req.Headers) > 0 {
		fmt.Fprintf(&b, "\n%s\n", m.theme.Muted.Render(" headers"))
		for _, h := range m.req.Headers {
			fmt.Fprintf(&b, "   %s: %s\n", h.Name, h.Value)
		}
	}
	if m.req.Auth.Type != core.AuthNone {
		fmt.Fprintf(&b, "\n%s %s\n", m.theme.Muted.Render(" auth"), m.req.Auth.Type)
	}
	if !m.req.Body.IsZero() {
		fmt.Fprintf(&b, "\n%s\n", m.theme.Muted.Render(" body ("+m.req.Body.Type+")"))
		for line := range strings.SplitSeq(m.req.Body.Content, "\n") {
			fmt.Fprintf(&b, "   %s\n", line)
		}
	}

	fmt.Fprintf(&b, "\n%s", m.theme.Muted.Render(" editing lands in milestone 3"))
	return b.String()
}
