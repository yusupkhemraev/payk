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
	"github.com/charmbracelet/x/ansi"

	"github.com/yusupkhemraev/payk/internal/httpc"
	"github.com/yusupkhemraev/payk/internal/tui/keymap"
	"github.com/yusupkhemraev/payk/internal/tui/theme"
)

type respTab int

const (
	respBody respTab = iota
	respHeaders
	respTimings
	respHistory
)

var respTabNames = []string{"Body", "Headers", "Timings", "History"}

const maxHistory = 50

type historyEntry struct {
	label string
	at    time.Time
	resp  *httpc.Response
	err   error
}

// highlightLimit caps syntax highlighting; larger bodies render as plain
// text so the UI never freezes on huge responses.
const highlightLimit = 200 * 1024

const prettyLimit = 2 << 20

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

	// wrap soft-wraps long lines in the body and headers tabs.
	wrap bool
	// raw switches the body tab to the unformatted response bytes.
	raw      bool
	rawLines []string
	// lineOffsets maps original body line index to wrapped viewport line.
	lineOffsets  []int
	headersLines []string
	timingsLines []string

	history []historyEntry
	// histCursor selects a row on the History tab, 0 = newest.
	histCursor int
}

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

func (m *Response) StartSending() tea.Cmd {
	m.sending = true
	m.err = nil
	return m.spin.Tick
}

func (m *Response) SetResponse(label string, resp *httpc.Response, err error) {
	m.history = append(m.history, historyEntry{label: label, at: time.Now(), resp: resp, err: err})
	if len(m.history) > maxHistory {
		m.history = m.history[len(m.history)-maxHistory:]
	}
	m.histCursor = 0
	m.loadResponse(resp, err)
}

func (m *Response) loadResponse(resp *httpc.Response, err error) {
	m.sending = false
	m.resp = resp
	m.err = err
	m.tab = respBody
	m.closeSearch()
	m.query = ""
	m.matches = nil
	m.raw = false
	if resp != nil {
		plain := m.plainBody(resp)
		m.plainLines = strings.Split(plain, "\n")
		m.renderedLines = strings.Split(m.highlightBody(resp, plain), "\n")
		m.rawLines = strings.Split(string(resp.Body), "\n")
		m.headersLines = m.buildHeaderLines(resp)
		m.timingsLines = m.buildTimingLines(resp)
		m.syncViewport()
		m.vp.GotoTop()
	}
}

func (m *Response) activePlain() []string {
	if m.raw {
		return m.rawLines
	}
	return m.plainLines
}

func (m *Response) activeRendered() []string {
	if m.raw {
		return m.rawLines
	}
	return m.renderedLines
}

func (m *Response) setTab(tab respTab) {
	if m.tab == tab {
		return
	}
	m.tab = tab
	m.syncViewport()
	m.vp.GotoTop()
}

func (m *Response) closeSearch() {
	m.searching = false
	m.searchInput.Blur()
	m.syncViewportHeight()
}

func (m *Response) searchBarVisible() bool {
	return m.searching || (m.query != "" && len(m.matches) > 0)
}

func (m *Response) syncViewportHeight() {
	// Frame with title and separator (4) + tab bar (2) + status line (2).
	height := m.height - 8
	if m.tab == respBody && m.searchBarVisible() {
		height--
	}
	m.vp.SetHeight(max(height, 1))
}

func (m *Response) syncViewport() {
	m.syncViewportHeight()
	switch m.tab {
	case respHeaders:
		m.vp.SetContentLines(m.wrapLines(m.headersLines, nil))
	case respTimings:
		m.vp.SetContentLines(m.timingsLines)
	case respHistory:
		m.vp.SetContentLines(m.historyLines())
		m.scrollHistoryCursorIntoView()
	default:
		m.vp.SetContentLines(m.bodyLines())
	}
}

// historyAt maps a display row (0 = newest) to its entry.
func (m *Response) historyAt(row int) historyEntry {
	return m.history[len(m.history)-1-row]
}

func (m *Response) historyLines() []string {
	if len(m.history) == 0 {
		return []string{m.theme.Muted.Render(" no requests sent yet")}
	}
	lines := make([]string, len(m.history))
	for row := range m.history {
		entry := m.historyAt(row)

		status := "error"
		statusStyle := m.theme.Status(500)
		meta := ""
		if entry.err == nil && entry.resp != nil {
			status = entry.resp.Status
			statusStyle = m.theme.Status(entry.resp.StatusCode)
			meta = fmt.Sprintf("  %s · %s",
				formatSize(entry.resp.Size()), formatDuration(entry.resp.Timings.Total))
		}

		if row == m.histCursor && m.focused {
			line := fmt.Sprintf(" %s  %-9s %s%s",
				entry.at.Format("15:04:05"), status, entry.label, meta)
			if pad := m.vp.Width() - len([]rune(line)); pad > 0 {
				line += strings.Repeat(" ", pad)
			}
			lines[row] = m.theme.Selected.Render(line)
			continue
		}
		lines[row] = fmt.Sprintf(" %s  %s %s%s",
			m.theme.Muted.Render(entry.at.Format("15:04:05")),
			statusStyle.Render(fmt.Sprintf("%-9s", status)),
			entry.label,
			m.theme.Muted.Render(meta))
	}
	return lines
}

func (m *Response) scrollHistoryCursorIntoView() {
	height := m.vp.VisibleLineCount()
	if m.histCursor < m.vp.YOffset() {
		m.vp.SetYOffset(m.histCursor)
	} else if m.histCursor >= m.vp.YOffset()+height {
		m.vp.SetYOffset(m.histCursor - height + 1)
	}
}

func (m *Response) bodyLines() []string {
	lines := m.activeRendered()
	if len(m.matches) > 0 && m.query != "" {
		lines = append([]string(nil), lines...)
		current := m.matches[m.matchIdx]
		if current < len(lines) {
			lines[current] = m.theme.Selected.Render(m.activePlain()[current])
		}
	}
	return m.wrapLines(lines, &m.lineOffsets)
}

func (m *Response) wrapLines(lines []string, offsets *[]int) []string {
	if !m.wrap {
		if offsets != nil {
			*offsets = nil
		}
		return lines
	}
	width := max(m.vp.Width(), 10)
	var wrapped []string
	if offsets != nil {
		*offsets = make([]int, len(lines))
	}
	for i, line := range lines {
		if offsets != nil {
			(*offsets)[i] = len(wrapped)
		}
		wrapped = append(wrapped, strings.Split(ansi.Hardwrap(line, width, true), "\n")...)
	}
	return wrapped
}

func (m *Response) computeMatches() {
	m.matches = m.matches[:0]
	m.matchIdx = 0
	query := strings.ToLower(m.query)
	if query == "" {
		return
	}
	for i, line := range m.activePlain() {
		if strings.Contains(strings.ToLower(line), query) {
			m.matches = append(m.matches, i)
		}
	}
}

func (m *Response) jumpToMatch() {
	m.syncViewport()
	if len(m.matches) == 0 {
		return
	}
	target := m.matches[m.matchIdx]
	if m.wrap && target < len(m.lineOffsets) {
		target = m.lineOffsets[target]
	}
	m.vp.SetYOffset(max(target-2, 0))
}

// SetSize sets the outer box size, borders included.
func (m *Response) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.vp.SetWidth(max(width-4, 10))
	m.syncViewportHeight()
}

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

	case key.Matches(msg, m.keys.Wrap):
		m.wrap = !m.wrap
		m.syncViewport()

	case key.Matches(msg, m.keys.Raw):
		if m.resp != nil && m.tab == respBody {
			m.raw = !m.raw
			m.computeMatches()
			m.syncViewport()
			m.vp.GotoTop()
		}

	case key.Matches(msg, m.keys.Yank):
		if m.resp != nil {
			body := strings.Join(m.activePlain(), "\n")
			return m, tea.Batch(
				tea.SetClipboard(body),
				noteCmd(fmt.Sprintf("copied %s to clipboard", formatSize(len(body))), false),
			)
		}

	case key.Matches(msg, m.keys.TabNext):
		m.setTab(respTab((int(m.tab) + 1) % len(respTabNames)))
	case key.Matches(msg, m.keys.TabPrev):
		m.setTab(respTab((int(m.tab) + len(respTabNames) - 1) % len(respTabNames)))

	// The History tab moves a selection instead of scrolling; enter loads
	// the selected past response back into the viewer.
	case m.tab == respHistory && key.Matches(msg, m.keys.Down):
		if m.histCursor < len(m.history)-1 {
			m.histCursor++
		}
		m.syncViewport()
	case m.tab == respHistory && key.Matches(msg, m.keys.Up):
		if m.histCursor > 0 {
			m.histCursor--
		}
		m.syncViewport()
	case m.tab == respHistory && key.Matches(msg, m.keys.Select):
		if len(m.history) > 0 {
			entry := m.historyAt(m.histCursor)
			m.loadResponse(entry.resp, entry.err)
		}

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

func (m Response) View() string {
	meta := ""
	if pos := m.scrollIndicator(); pos != "" {
		meta = strings.TrimPrefix(pos, "· ")
	}
	return frame(m.theme, "RESPONSE", meta, m.focused, m.width, m.height, m.body())
}

func (m Response) body() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", m.tabBar())

	switch {
	case m.sending:
		fmt.Fprintf(&b, " %s sending… %s", m.spin.View(), m.theme.Muted.Render("(esc to cancel)"))
	case m.tab == respHistory:
		b.WriteString(m.vp.View())
		if len(m.history) > 0 {
			fmt.Fprintf(&b, "\n %s", m.theme.Muted.Render("enter to open · j/k select"))
		}
	case m.err != nil:
		m.renderError(&b)
	case m.resp == nil:
		fmt.Fprintf(&b, " %s", m.theme.Muted.Render("no response yet — press space to send"))
	default:
		m.renderResponse(&b)
	}
	return b.String()
}

func (m Response) renderError(b *strings.Builder) {
	const prefix = " error "
	limit := max(m.width-2-len(prefix), 16)
	wrapped := strings.Split(ansi.Wrap(m.err.Error(), limit, ""), "\n")

	fmt.Fprintf(b, " %s %s", m.theme.Status(500).Render("error"), wrapped[0])
	indent := strings.Repeat(" ", len(prefix))
	for _, line := range wrapped[1:] {
		fmt.Fprintf(b, "\n%s%s", indent, line)
	}
}

func (m Response) renderResponse(b *strings.Builder) {
	fmt.Fprintf(b, "%s\n\n", m.statusLine())
	if m.tab == respBody {
		if line := m.searchLine(); line != "" {
			fmt.Fprintf(b, "%s\n", line)
		}
	}
	b.WriteString(m.vp.View())
}

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

// statusLine renders the status pill plus response metadata; scroll position
// lives in the pane header instead.
func (m Response) statusLine() string {
	pill := m.theme.Status(m.resp.StatusCode).
		Background(m.theme.Surface).
		Render(" " + m.resp.Status + " ")
	meta := fmt.Sprintf("%s  ·  %s  ·  %s",
		formatDuration(m.resp.Timings.Total), formatSize(m.resp.Size()), m.resp.Proto)
	line := " " + pill + "  " + m.theme.Muted.Render(meta)
	if m.resp.Truncated {
		line += "  " + m.theme.Status(400).Render("truncated")
	}
	if len(m.history) > 0 && m.theme.Icons.History != "" {
		line = SplitRow(line,
			m.theme.Muted.Render(fmt.Sprintf("%s %d", m.theme.Icons.History, len(m.history))),
			m.width-2)
	}
	return line
}

func (m Response) scrollIndicator() string {
	total := m.vp.TotalLineCount()
	visible := m.vp.VisibleLineCount()
	var parts []string
	if total > visible {
		from := m.vp.YOffset() + 1
		to := min(m.vp.YOffset()+visible, total)
		parts = append(parts, fmt.Sprintf("· %d–%d/%d (%d%%)",
			from, to, total, int(m.vp.ScrollPercent()*100)))
	}
	if m.wrap {
		parts = append(parts, "· wrap")
	}
	if m.raw {
		parts = append(parts, "· raw")
	}
	return strings.Join(parts, " ")
}

func (m Response) tabBar() string {
	var parts []string
	for i, name := range respTabNames {
		if respTab(i) == respHeaders && m.resp != nil {
			name = fmt.Sprintf("%s (%d)", name, len(m.resp.Headers))
		}
		if respTab(i) == respHistory && len(m.history) > 0 {
			name = fmt.Sprintf("%s (%d)", name, len(m.history))
		}
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

func (m Response) buildHeaderLines(resp *httpc.Response) []string {
	names := make([]string, 0, len(resp.Headers))
	nameWidth := 0
	for name := range resp.Headers {
		names = append(names, name)
		nameWidth = max(nameWidth, len(name))
	}
	sort.Strings(names)
	nameWidth = min(nameWidth, 30)

	var lines []string
	for _, name := range names {
		for _, value := range resp.Headers[name] {
			padded := fmt.Sprintf("%-*s", nameWidth, name)
			lines = append(lines, " "+m.theme.FieldLabel.Render(padded)+"  "+value)
		}
	}
	return lines
}

func (m Response) buildTimingLines(resp *httpc.Response) []string {
	t := resp.Timings
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
	var lines []string
	for _, row := range rows {
		value := "—"
		if row.value > 0 {
			value = formatDuration(row.value)
		}
		lines = append(lines, " "+m.theme.FieldLabel.Render(fmt.Sprintf("%-12s", row.label))+" "+value)
	}
	lines = append(lines, "",
		" "+m.theme.FieldLabel.Render(fmt.Sprintf("%-12s", "Size"))+" "+formatSize(resp.Size()),
		" "+m.theme.FieldLabel.Render(fmt.Sprintf("%-12s", "Status"))+" "+resp.Status)
	return lines
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
