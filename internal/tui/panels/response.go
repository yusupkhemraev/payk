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
	"charm.land/bubbles/v2/textinput"
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

	searching   bool
	searchInput textinput.Model
	query       string
	// plainLines is the un-highlighted body used for matching; rendered
	// lines carry ANSI colors and line up with it 1:1.
	plainLines    []string
	renderedLines []string
	matches       []int
	matchIdx      int
}

// NewResponse builds the response viewer panel.
func NewResponse(t *theme.Theme, keys keymap.KeyMap) Response {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	search := textinput.New()
	search.Prompt = "/"
	return Response{theme: t, keys: keys, spin: sp, vp: viewport.New(), searchInput: search}
}

// Searching reports whether the search input captures keystrokes; the root
// model must not treat keys as global shortcuts while true.
func (m *Response) Searching() bool {
	return m.searching
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
	m.closeSearch()
	m.query = ""
	m.matches = nil
	if resp != nil {
		plain := m.plainBody(resp)
		m.plainLines = strings.Split(plain, "\n")
		m.renderedLines = strings.Split(m.highlightBody(resp, plain), "\n")
		m.syncViewport()
		m.vp.GotoTop()
	}
}

func (m *Response) closeSearch() {
	m.searching = false
	m.searchInput.Blur()
	m.syncViewportHeight()
}

// searchBarVisible reports whether a search line occupies a viewport row.
func (m *Response) searchBarVisible() bool {
	return m.searching || (m.query != "" && len(m.matches) > 0)
}

func (m *Response) syncViewportHeight() {
	// Frame (3) + tab bar (2) + status line (2) rows around the viewport.
	height := m.height - 7
	if m.searchBarVisible() {
		height--
	}
	m.vp.SetHeight(max(height, 1))
}

// syncViewport rebuilds the viewport content, highlighting the current
// search match line.
func (m *Response) syncViewport() {
	m.syncViewportHeight()
	if len(m.matches) == 0 || m.query == "" {
		m.vp.SetContentLines(m.renderedLines)
		return
	}
	lines := append([]string(nil), m.renderedLines...)
	current := m.matches[m.matchIdx]
	if current < len(lines) {
		lines[current] = m.theme.Selected.Render(m.plainLines[current])
	}
	m.vp.SetContentLines(lines)
}

func (m *Response) computeMatches() {
	m.matches = m.matches[:0]
	m.matchIdx = 0
	query := strings.ToLower(m.query)
	if query == "" {
		return
	}
	for i, line := range m.plainLines {
		if strings.Contains(strings.ToLower(line), query) {
			m.matches = append(m.matches, i)
		}
	}
}

// jumpToMatch scrolls the current match into view with a little context.
func (m *Response) jumpToMatch() {
	m.syncViewport()
	if len(m.matches) == 0 {
		return
	}
	m.vp.SetYOffset(max(m.matches[m.matchIdx]-2, 0))
}

// SetSize sets the outer box size, borders included.
func (m *Response) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.vp.SetWidth(max(width-4, 10))
	m.syncViewportHeight()
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
	if m.searching {
		return m.updateSearch(msg)
	}

	if m.pendingG {
		m.pendingG = false
		if msg.String() == "g" {
			m.vp.GotoTop()
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Search):
		if m.resp != nil && m.tab == respBody {
			m.searching = true
			m.searchInput.SetValue(m.query)
			m.searchInput.CursorEnd()
			m.syncViewportHeight()
			return m, m.searchInput.Focus()
		}

	case key.Matches(msg, m.keys.SearchNext):
		m.cycleMatch(1)
	case key.Matches(msg, m.keys.SearchPrev):
		m.cycleMatch(-1)

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

// updateSearch drives the "/" input: live matching while typing, enter keeps
// the query for n/N, esc clears it.
func (m Response) updateSearch(msg tea.KeyPressMsg) (Response, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.query = ""
		m.matches = nil
		m.closeSearch()
		m.syncViewport()
		return m, nil

	case msg.Code == tea.KeyEnter:
		m.closeSearch()
		m.syncViewport()
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.query = m.searchInput.Value()
	m.computeMatches()
	m.jumpToMatch()
	return m, cmd
}

func (m *Response) cycleMatch(delta int) {
	if len(m.matches) == 0 {
		return
	}
	m.matchIdx = (m.matchIdx + delta + len(m.matches)) % len(m.matches)
	m.jumpToMatch()
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
		if line := m.searchLine(); line != "" {
			fmt.Fprintf(b, "%s\n", line)
		}
		b.WriteString(m.vp.View())
	case respHeaders:
		m.renderHeaders(b)
	case respTimings:
		m.renderTimings(b)
	}
}

// searchLine renders the "/" input or the committed query with match count.
func (m Response) searchLine() string {
	switch {
	case m.searching:
		count := ""
		if m.query != "" {
			count = m.theme.Muted.Render(fmt.Sprintf("  %d matches", len(m.matches)))
		}
		return " " + m.searchInput.View() + count
	case m.query != "" && len(m.matches) > 0:
		return " " + m.theme.Muted.Render(fmt.Sprintf("/%s  %d/%d  n/N to cycle",
			m.query, m.matchIdx+1, len(m.matches)))
	}
	return ""
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

// plainBody pretty-prints the body without styling; search matches against
// these lines.
func (m Response) plainBody(resp *httpc.Response) string {
	body := resp.Body
	if len(body) == 0 {
		return "(empty body)"
	}
	if looksLikeJSON(resp) && len(body) <= prettyLimit {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, bytes.TrimSpace(body), "", "  "); err == nil {
			return pretty.String()
		}
	}
	return string(body)
}

// highlightBody colors the plain body, capped by size so large responses
// never block the UI. Line structure must stay identical to plainBody.
func (m Response) highlightBody(resp *httpc.Response, plain string) string {
	if len(resp.Body) == 0 {
		return m.theme.Muted.Render("(empty body)")
	}
	if looksLikeJSON(resp) && len(plain) <= highlightLimit {
		var highlighted bytes.Buffer
		err := quick.Highlight(&highlighted, plain, "json", "terminal16m", m.theme.ChromaStyle())
		if err == nil {
			return strings.TrimSuffix(highlighted.String(), "\n")
		}
	}
	return plain
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
