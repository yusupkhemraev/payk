package panels

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/alecthomas/chroma/v2/quick"

	"github.com/yusupkhemraev/payk/internal/httpc"
	"github.com/yusupkhemraev/payk/internal/tui/keymap"
	"github.com/yusupkhemraev/payk/internal/tui/theme"
)

type respTab int

const (
	respBody respTab = iota
	respHeaders
	respTimings
)

var respTabNames = []string{"Body", "Headers", "Timings"}

// highlightLimit caps syntax highlighting; larger bodies render as plain
// text so the UI never freezes on huge responses.
const highlightLimit = 200 * 1024

// prettyLimit caps JSON pretty-printing.
const prettyLimit = 2 << 20

// Response is the right panel: response body, headers, and timings.
type Response struct {
	theme *theme.Theme
	keys  keymap.KeyMap

	width   int
	height  int
	focused bool

	tab      respTab
	sending  bool
	spin     spinner.Model
	resp     *httpc.Response
	err      error
	vp       viewport.Model
	pendingG bool
}

// NewResponse builds the response viewer panel.
func NewResponse(t *theme.Theme, keys keymap.KeyMap) Response {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	return Response{theme: t, keys: keys, spin: sp, vp: viewport.New()}
}

// StartSending switches to the in-flight state and starts the spinner.
func (m *Response) StartSending() tea.Cmd {
	m.sending = true
	m.err = nil
	return m.spin.Tick
}

// SetResponse stores the send result and renders the body into the viewport.
func (m *Response) SetResponse(resp *httpc.Response, err error) {
	m.sending = false
	m.resp = resp
	m.err = err
	m.tab = respBody
	if resp != nil {
		m.vp.SetContent(m.renderBody(resp))
		m.vp.GotoTop()
	}
}

// SetSize sets the outer box size, borders included.
func (m *Response) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.vp.SetWidth(max(width-4, 10))
	// Frame (3) + tab bar (2) + status line (2) rows around the viewport.
	m.vp.SetHeight(max(height-7, 1))
}

// SetFocused toggles keyboard focus for this panel.
func (m *Response) SetFocused(focused bool) {
	m.focused = focused
	if !focused {
		m.pendingG = false
	}
}

// Update handles spinner ticks always and scroll keys while focused.
func (m Response) Update(msg tea.Msg) (Response, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !m.sending {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Response) handleKey(msg tea.KeyPressMsg) (Response, tea.Cmd) {
	if m.pendingG {
		m.pendingG = false
		if msg.String() == "g" {
			m.vp.GotoTop()
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.TabNext):
		m.tab = respTab((int(m.tab) + 1) % len(respTabNames))
	case key.Matches(msg, m.keys.TabPrev):
		m.tab = respTab((int(m.tab) + len(respTabNames) - 1) % len(respTabNames))
	case key.Matches(msg, m.keys.Down):
		m.vp.ScrollDown(1)
	case key.Matches(msg, m.keys.Up):
		m.vp.ScrollUp(1)
	case key.Matches(msg, m.keys.HalfDown):
		m.vp.HalfPageDown()
	case key.Matches(msg, m.keys.HalfUp):
		m.vp.HalfPageUp()
	case key.Matches(msg, m.keys.Bottom):
		m.vp.GotoBottom()
	case key.Matches(msg, m.keys.Top):
		m.pendingG = true
	}
	return m, nil
}

// View renders the panel at its current size.
func (m Response) View() string {
	return frame(m.theme, "Response", m.focused, m.width, m.height, m.body())
}

func (m Response) body() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", m.tabBar())

	switch {
	case m.sending:
		fmt.Fprintf(&b, " %s sending… %s", m.spin.View(), m.theme.Muted.Render("(esc to cancel)"))
	case m.err != nil:
		fmt.Fprintf(&b, " %s %s", m.theme.Status(500).Render("error"), m.err.Error())
	case m.resp == nil:
		fmt.Fprintf(&b, " %s", m.theme.Muted.Render("no response yet — press space to send"))
	default:
		m.renderResponse(&b)
	}
	return b.String()
}

func (m Response) renderResponse(b *strings.Builder) {
	fmt.Fprintf(b, "%s\n\n", m.statusLine())

	switch m.tab {
	case respBody:
		b.WriteString(m.vp.View())
	case respHeaders:
		m.renderHeaders(b)
	case respTimings:
		m.renderTimings(b)
	}
}

func (m Response) statusLine() string {
	status := m.theme.Status(m.resp.StatusCode).Render(m.resp.Status)
	meta := fmt.Sprintf("%s · %s · %s",
		m.resp.Proto, formatSize(m.resp.Size()), formatDuration(m.resp.Timings.Total))
	line := " " + status + " " + m.theme.Muted.Render(meta)
	if m.resp.Truncated {
		line += " " + m.theme.Status(400).Render("(truncated)")
	}
	return line
}

func (m Response) tabBar() string {
	var parts []string
	for i, name := range respTabNames {
		style := m.theme.TabInactive
		if respTab(i) == m.tab {
			style = m.theme.TabActive
		}
		parts = append(parts, style.Render(name))
	}
	return " " + strings.Join(parts, m.theme.TabInactive.Render(" · "))
}

// renderBody pretty-prints and highlights the body, capped by size so large
// responses never block the UI.
func (m Response) renderBody(resp *httpc.Response) string {
	body := resp.Body
	if len(body) == 0 {
		return m.theme.Muted.Render("(empty body)")
	}

	isJSON := looksLikeJSON(resp)
	if isJSON && len(body) <= prettyLimit {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, bytes.TrimSpace(body), "", "  "); err == nil {
			body = pretty.Bytes()
		}
	}

	if isJSON && len(body) <= highlightLimit {
		var highlighted bytes.Buffer
		err := quick.Highlight(&highlighted, string(body), "json", "terminal16m", m.theme.ChromaStyle())
		if err == nil {
			return highlighted.String()
		}
	}
	return string(body)
}

func looksLikeJSON(resp *httpc.Response) bool {
	if strings.Contains(resp.Headers.Get("Content-Type"), "json") {
		return true
	}
	trimmed := bytes.TrimSpace(resp.Body)
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}

func (m Response) renderHeaders(b *strings.Builder) {
	names := make([]string, 0, len(m.resp.Headers))
	for name := range m.resp.Headers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		for _, value := range m.resp.Headers[name] {
			fmt.Fprintf(b, " %s %s\n", m.theme.FieldLabel.Render(name+":"), value)
		}
	}
}

func (m Response) renderTimings(b *strings.Builder) {
	t := m.resp.Timings
	rows := []struct {
		label string
		value time.Duration
	}{
		{"DNS", t.DNS},
		{"TCP connect", t.Connect},
		{"TLS", t.TLS},
		{"TTFB", t.TTFB},
		{"Total", t.Total},
	}
	for _, row := range rows {
		value := "—"
		if row.value > 0 {
			value = formatDuration(row.value)
		}
		fmt.Fprintf(b, " %s %s\n", m.theme.FieldLabel.Render(fmt.Sprintf("%-12s", row.label)), value)
	}
	fmt.Fprintf(b, "\n %s %s\n", m.theme.FieldLabel.Render(fmt.Sprintf("%-12s", "Size")), formatSize(m.resp.Size()))
	fmt.Fprintf(b, " %s %s\n", m.theme.FieldLabel.Render(fmt.Sprintf("%-12s", "Status")), m.resp.Status)
}

func formatDuration(d time.Duration) string {
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%.2fs", d.Seconds())
	case d >= time.Millisecond:
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	default:
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
}

func formatSize(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
