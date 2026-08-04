package panels

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2/quick"

	"github.com/yusupkhemraev/payk/internal/core"
	"github.com/yusupkhemraev/payk/internal/tui/keymap"
	"github.com/yusupkhemraev/payk/internal/tui/theme"
)

type editorTab int

const (
	tabBody editorTab = iota
	tabParams
	tabHeaders
	tabAuth
	tabInfo
)

var editorTabNames = []string{"Body", "Params", "Headers", "Auth", "Info"}

// urlRows are the method and URL: they live on the line above the tabs and
// stay reachable whichever tab is open, so rows 0 and 1 always mean them.
const urlRows = 2

var methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

var bodyTypes = []string{"json", "text", "form"}

var authTypes = []string{core.AuthNone, core.AuthBearer, core.AuthBasic}

// Request edits mutate the in-memory request; :w persists.
type Request struct {
	theme *theme.Theme
	keys  keymap.KeyMap

	width   int
	height  int
	focused bool

	req *core.Request
	tab editorTab
	row int
	// insert reports whether an input field captures keystrokes; the root
	// model stops handling global keys while it is set.
	insert bool

	urlInput  textinput.Model
	kvName    textinput.Model
	kvValue   textinput.Model
	kvField   int
	bodyArea  textarea.Model
	authInput textinput.Model
	infoInput textinput.Model

	// envVars/osEnv feed {{var}} completion in insert mode.
	envVars []string
	osEnv   []string
	suggest *suggestState

	// collection/path locate the request for the breadcrumb; dirty tracks
	// edits made since the last :w.
	collection string
	path       []string
	dirty      bool
	// lineNumbers mirrors the config toggle for the body gutter.
	lineNumbers bool
}

func NewRequest(t *theme.Theme, keys keymap.KeyMap) Request {
	url := textinput.New()
	url.Prompt = ""

	name := textinput.New()
	name.Prompt = ""
	name.Placeholder = "name"

	value := textinput.New()
	value.Prompt = ""
	value.Placeholder = "value"

	auth := textinput.New()
	auth.Prompt = ""

	info := textinput.New()
	info.Prompt = ""
	info.Placeholder = "what this request does"

	body := textarea.New()

	return Request{
		theme:     t,
		keys:      keys,
		urlInput:  url,
		kvName:    name,
		kvValue:   value,
		bodyArea:  body,
		authInput: auth,
		infoInput: info,
	}
}

// SetTheme swaps the theme after a config reload.
func (m *Request) SetTheme(t *theme.Theme, lineNumbers bool) {
	m.theme = t
	m.lineNumbers = lineNumbers
}

func (m *Request) SetRequest(req *core.Request) {
	m.req = req
	m.tab = tabBody
	m.row = 0
	m.insert = false
	m.suggest = nil
	m.dirty = false
}

// SetEnvironment provides variable names for {{var}} completion: vars from
// the active environment and names from the process environment.
func (m *Request) SetEnvironment(envVars, osEnv []string) {
	m.envVars = envVars
	m.osEnv = osEnv
}

func (m *Request) CurrentRequest() *core.Request {
	return m.req
}

// Editing reports whether an input captures keystrokes; the root model must
// not treat keys as global shortcuts while true.
func (m *Request) Editing() bool {
	return m.insert
}

// SetSize sets the outer box size, borders included.
func (m *Request) SetSize(width, height int) {
	m.width = width
	m.height = height
	inner := max(width-4, 10)
	m.urlInput.SetWidth(inner)
	m.kvName.SetWidth(max(inner/3, 8))
	m.kvValue.SetWidth(max(inner/2, 8))
	m.authInput.SetWidth(inner)
	m.bodyArea.SetWidth(inner)
	m.bodyArea.SetHeight(max(height-8, 3))
}

func (m *Request) SetFocused(focused bool) {
	m.focused = focused
	if !focused && m.insert {
		m.commitInsert()
	}
}

// EditBodyRequestedMsg asks the root model to open the body in $EDITOR via
// tea.ExecProcess.
type EditBodyRequestedMsg struct {
	Content  string
	BodyType string
}

func (m *Request) SetBodyContent(content string) {
	if m.req != nil {
		m.dirty = true
		m.req.Body.Content = content
		if m.req.Body.Type == "" && content != "" {
			m.req.Body.Type = "json"
		}
	}
}

func (m *Request) rows() int {
	return urlRows + m.tabRows()
}

func (m *Request) tabRows() int {
	switch m.tab {
	case tabParams:
		return len(m.req.Params)
	case tabHeaders:
		return len(m.req.Headers)
	case tabBody:
		return 2 // type, content
	case tabInfo:
		return 1 // description
	case tabAuth:
		switch m.req.Auth.Type {
		case core.AuthBearer:
			return 2 // type, token
		case core.AuthBasic:
			return 3 // type, user, pass
		default:
			return 1 // type
		}
	}
	return 0
}

// onURLRow reports whether the selection sits on the method or URL line.
func (m *Request) onURLRow() bool {
	return m.row < urlRows
}

// tabRow is the selection index within the open tab.
func (m *Request) tabRow() int {
	return m.row - urlRows
}

func (m Request) Update(msg tea.Msg) (Request, tea.Cmd) {
	if m.req == nil {
		return m, nil
	}
	if m.insert {
		return m.updateInsert(msg)
	}

	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(keyMsg, m.keys.TabNext):
		m.tab = editorTab((int(m.tab) + 1) % len(editorTabNames))
		m.row = urlRows

	case key.Matches(keyMsg, m.keys.TabPrev):
		m.tab = editorTab((int(m.tab) + len(editorTabNames) - 1) % len(editorTabNames))
		m.row = urlRows

	case key.Matches(keyMsg, m.keys.Down):
		if m.row < m.rows()-1 {
			m.row++
		}

	case key.Matches(keyMsg, m.keys.Up):
		if m.row > 0 {
			m.row--
		}

	case key.Matches(keyMsg, m.keys.AddRow) && (m.tab == tabParams || m.tab == tabHeaders):
		return m.addRow()

	case key.Matches(keyMsg, m.keys.DeleteRow) && (m.tab == tabParams || m.tab == tabHeaders):
		m.deleteRow()

	case key.Matches(keyMsg, m.keys.Format) && m.tab == tabBody:
		return m.formatBody()

	case key.Matches(keyMsg, m.keys.Edit) && m.tab == tabBody:
		msg := EditBodyRequestedMsg{Content: m.req.Body.Content, BodyType: m.req.Body.Type}
		return m, func() tea.Msg { return msg }

	case key.Matches(keyMsg, m.keys.Select), key.Matches(keyMsg, m.keys.Insert):
		return m.activateRow()
	}
	return m, nil
}

func (m Request) formatBody() (Request, tea.Cmd) {
	trimmed := strings.TrimSpace(m.req.Body.Content)
	if trimmed == "" {
		return m, noteCmd("body is empty — nothing to format", true)
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(trimmed), "", "  "); err != nil {
		return m, noteCmd("body is not valid JSON: "+err.Error(), true)
	}
	m.req.Body.Content = pretty.String()
	m.dirty = true
	if m.req.Body.Type == "" {
		m.req.Body.Type = "json"
	}
	return m, noteCmd("body formatted", false)
}

func (m Request) addRow() (Request, tea.Cmd) {
	m.dirty = true
	kv := core.KV{}
	if m.tab == tabParams {
		m.req.Params = append(m.req.Params, kv)
		m.row = urlRows + len(m.req.Params) - 1
	} else {
		m.req.Headers = append(m.req.Headers, kv)
		m.row = urlRows + len(m.req.Headers) - 1
	}
	return m.startKVInsert()
}

func (m *Request) deleteRow() {
	m.dirty = true
	kvs := m.currentKVs()
	row := m.tabRow()
	if row < 0 || row >= len(*kvs) {
		return
	}
	*kvs = append((*kvs)[:row], (*kvs)[row+1:]...)
	if row >= len(*kvs) && m.row > urlRows {
		m.row--
	}
}

func (m *Request) currentKVs() *[]core.KV {
	if m.tab == tabParams {
		return &m.req.Params
	}
	return &m.req.Headers
}

func (m Request) activateRow() (Request, tea.Cmd) {
	if m.onURLRow() {
		if m.row == 0 {
			m.req.Method = cycle(methods, m.req.Method)
			m.dirty = true
			return m, nil
		}
		m.insert = true
		m.urlInput.SetValue(m.req.URL)
		m.urlInput.CursorEnd()
		m.refreshSuggestions()
		return m, m.urlInput.Focus()
	}

	switch m.tab {
	case tabParams, tabHeaders:
		if len(*m.currentKVs()) == 0 {
			return m.addRow()
		}
		return m.startKVInsert()

	case tabBody:
		if m.tabRow() == 0 {
			m.req.Body.Type = cycle(bodyTypes, m.req.Body.Type)
			m.dirty = true
			return m, nil
		}
		m.insert = true
		m.bodyArea.SetValue(m.req.Body.Content)
		return m, m.bodyArea.Focus()

	case tabInfo:
		m.insert = true
		m.infoInput.SetValue(m.req.Description)
		m.infoInput.CursorEnd()
		return m, m.infoInput.Focus()

	case tabAuth:
		if m.tabRow() == 0 {
			m.req.Auth.Type = cycle(authTypes, m.req.Auth.Type)
			m.dirty = true
			m.row = urlRows
			return m, nil
		}
		m.insert = true
		m.authInput.SetValue(m.authValue())
		m.authInput.CursorEnd()
		m.refreshSuggestions()
		return m, m.authInput.Focus()
	}
	return m, nil
}

func (m Request) startKVInsert() (Request, tea.Cmd) {
	kvs := *m.currentKVs()
	if m.tabRow() >= len(kvs) {
		return m, nil
	}
	m.insert = true
	m.kvField = 0
	m.kvName.SetValue(kvs[m.tabRow()].Name)
	m.kvName.CursorEnd()
	m.kvValue.SetValue(kvs[m.tabRow()].Value)
	m.kvValue.CursorEnd()
	m.kvValue.Blur()
	m.refreshSuggestions()
	return m, m.kvName.Focus()
}

func (m Request) updateInsert(msg tea.Msg) (Request, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Escape):
			m.commitInsert()
			return m, nil

		// With an open {{ completion, tab accepts and ctrl+n/p cycle.
		case m.suggest.active() && keyMsg.Code == tea.KeyTab && keyMsg.Mod == 0:
			m.acceptSuggestion()
			return m, nil
		case m.suggest.active() && (keyMsg.String() == "ctrl+n" || keyMsg.Code == tea.KeyDown):
			m.suggest.move(1)
			return m, nil
		case m.suggest.active() && (keyMsg.String() == "ctrl+p" || keyMsg.Code == tea.KeyUp):
			m.suggest.move(-1)
			return m, nil

		case (m.tab == tabParams || m.tab == tabHeaders) &&
			(key.Matches(keyMsg, m.keys.NextPane) || key.Matches(keyMsg, m.keys.PrevPane)):
			// tab/shift+tab jump between the name and value inputs.
			if m.kvField == 0 {
				m.kvField = 1
				m.kvName.Blur()
				m.refreshSuggestions()
				return m, m.kvValue.Focus()
			}
			m.kvField = 0
			m.kvValue.Blur()
			m.refreshSuggestions()
			return m, m.kvName.Focus()

		// On KV tabs enter commits the row and chains into a fresh one, so
		// several headers/params can be typed in a row; esc stops.
		case (m.tab == tabParams || m.tab == tabHeaders) && keyMsg.Code == tea.KeyEnter:
			m.commitInsert()
			kvs := *m.currentKVs()
			row := m.tabRow()
			if row == len(kvs)-1 && row >= 0 &&
				(kvs[row].Name != "" || kvs[row].Value != "") {
				return m.addRow()
			}
			return m, nil

		case (m.onURLRow() || m.tab != tabBody) && keyMsg.Code == tea.KeyEnter:
			m.commitInsert()
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.onURLRow() {
		m.urlInput, cmd = m.urlInput.Update(msg)
		m.refreshSuggestions()
		return m, cmd
	}
	switch m.tab {
	case tabInfo:
		m.infoInput, cmd = m.infoInput.Update(msg)
	case tabParams, tabHeaders:
		if m.kvField == 0 {
			m.kvName, cmd = m.kvName.Update(msg)
		} else {
			m.kvValue, cmd = m.kvValue.Update(msg)
		}
	case tabBody:
		m.bodyArea, cmd = m.bodyArea.Update(msg)
	case tabAuth:
		m.authInput, cmd = m.authInput.Update(msg)
	}
	m.refreshSuggestions()
	return m, cmd
}

// activeInput returns the focused textinput of the current insert session,
// or nil for the body textarea (no completion there).
func (m *Request) activeInput() *textinput.Model {
	if m.onURLRow() {
		return &m.urlInput
	}
	switch m.tab {
	case tabParams, tabHeaders:
		if m.kvField == 0 {
			return &m.kvName
		}
		return &m.kvValue
	case tabAuth:
		return &m.authInput
	}
	return nil
}

func (m *Request) refreshSuggestions() {
	input := m.activeInput()
	if input == nil {
		m.suggest = nil
		return
	}
	prev := m.suggest
	m.suggest = varSuggestions(input.Value(), input.Position(), m.envVars, m.osEnv)
	// Keep the highlighted entry stable while the match list is unchanged.
	if prev != nil && m.suggest != nil && prev.index < len(m.suggest.matches) &&
		len(prev.matches) == len(m.suggest.matches) {
		m.suggest.index = prev.index
	}
}

func (m *Request) acceptSuggestion() {
	input := m.activeInput()
	if input == nil || !m.suggest.active() {
		return
	}
	value, cursor := m.suggest.apply(input.Value(), input.Position())
	input.SetValue(value)
	input.SetCursor(cursor)
	m.refreshSuggestions()
}

func (m Request) suggestionLine() string {
	if !m.suggest.active() {
		return ""
	}
	var parts []string
	for i, name := range m.suggest.matches {
		label := "{{" + name + "}}"
		if name == "env:" {
			label = "{{env:…}}"
		}
		if i == m.suggest.index {
			parts = append(parts, m.theme.Selected.Render(label))
		} else {
			parts = append(parts, m.theme.Muted.Render(label))
		}
	}
	return " " + m.theme.FieldLabel.Render("↹") + " " + strings.Join(parts, " ")
}

func (m *Request) commitInsert() {
	m.dirty = true
	if m.onURLRow() {
		m.req.URL = strings.TrimSpace(m.urlInput.Value())
		m.finishInsert()
		return
	}

	switch m.tab {
	case tabInfo:
		m.req.Description = strings.TrimSpace(m.infoInput.Value())
	case tabParams, tabHeaders:
		kvs := m.currentKVs()
		row := m.tabRow()
		if row >= 0 && row < len(*kvs) {
			kv := core.KV{
				Name:  strings.TrimSpace(m.kvName.Value()),
				Value: m.kvValue.Value(),
			}
			if kv.Name == "" && kv.Value == "" {
				*kvs = append((*kvs)[:row], (*kvs)[row+1:]...)
				if m.row > urlRows {
					m.row--
				}
			} else {
				(*kvs)[row] = kv
			}
		}
	case tabBody:
		m.req.Body.Content = m.bodyArea.Value()
		if m.req.Body.Type == "" && m.req.Body.Content != "" {
			m.req.Body.Type = "json"
		}
	case tabAuth:
		m.setAuthValue(strings.TrimSpace(m.authInput.Value()))
	}
	m.finishInsert()
}

func (m *Request) finishInsert() {
	m.insert = false
	m.suggest = nil
	m.urlInput.Blur()
	m.kvName.Blur()
	m.kvValue.Blur()
	m.bodyArea.Blur()
	m.authInput.Blur()
	m.infoInput.Blur()
}

func (m *Request) authValue() string {
	switch {
	case m.req.Auth.Type == core.AuthBearer:
		return m.req.Auth.Token
	case m.tabRow() == 1:
		return m.req.Auth.User
	default:
		return m.req.Auth.Pass
	}
}

func (m *Request) setAuthValue(v string) {
	switch {
	case m.req.Auth.Type == core.AuthBearer:
		m.req.Auth.Token = v
	case m.tabRow() == 1:
		m.req.Auth.User = v
	default:
		m.req.Auth.Pass = v
	}
}

func cycle(values []string, current string) string {
	for i, v := range values {
		if v == current {
			return values[(i+1)%len(values)]
		}
	}
	return values[0]
}

// SetLocation records where the open request lives, for the breadcrumb.
func (m *Request) SetLocation(collection string, path []string) {
	m.collection = collection
	m.path = path
}

// MarkDirty flags unsaved edits, shown in the breadcrumb and the tree.
func (m *Request) MarkDirty(dirty bool) {
	m.dirty = dirty
}

// Dirty reports whether the open request has unsaved edits.
func (m *Request) Dirty() bool {
	return m.dirty
}

// breadcrumb shows the open request's location plus an unsaved marker.
func (m Request) breadcrumb() string {
	if m.req == nil {
		return ""
	}
	parts := append([]string{m.collection}, m.path...)
	parts = append(parts, m.req.Name)
	crumb := strings.Join(parts, " / ")
	if m.dirty {
		crumb += "  " + m.theme.Dirty.Render(m.dirtyMark())
	}
	return crumb
}

func (m Request) dirtyMark() string {
	if mark := m.theme.Icons.Dirty; mark != "" {
		return mark
	}
	return "*"
}

func (m Request) View() string {
	return frame(m.theme, "Request", m.breadcrumb(), m.focused, m.width, m.height, m.body())
}

func (m Request) body() string {
	if m.req == nil {
		return m.tabBar() + "\n\n" + m.theme.Muted.Render(" select a request in collections (enter)")
	}

	var b strings.Builder
	m.renderURLLine(&b)
	fmt.Fprintf(&b, "\n%s\n\n", m.tabBar())

	switch m.tab {
	case tabParams:
		m.renderKVTab(&b, m.req.Params, "params — a to add")
	case tabHeaders:
		m.renderKVTab(&b, m.req.Headers, "headers — a to add")
	case tabBody:
		m.renderBodyTab(&b)
	case tabAuth:
		m.renderAuthTab(&b)
	case tabInfo:
		m.renderInfoTab(&b)
	}

	fmt.Fprintf(&b, "\n%s", m.theme.Muted.Render(" "+m.hint()))
	return b.String()
}

func (m Request) hint() string {
	if m.insert {
		if m.suggest.active() {
			return "tab complete · ctrl+n/p cycle · esc done"
		}
		if m.tab == tabParams || m.tab == tabHeaders {
			return "tab name/value · enter next row · esc done"
		}
		return "{{ vars · esc done"
	}
	return "j/k rows · [ ] tabs · i edit · space send"
}

func (m Request) tabBar() string {
	var parts []string
	for i, name := range editorTabNames {
		style := m.theme.TabInactive
		if editorTab(i) == m.tab {
			style = m.theme.TabActive
		}
		label := style.Render(name)
		if m.tabFilled(editorTab(i)) {
			label += " " + m.theme.TabDot.Render("•")
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, "   ")
}

// tabFilled reports whether a tab holds anything, mirrored by a dot next to
// its name so empty sections are obvious without opening them.
func (m Request) tabFilled(tab editorTab) bool {
	if m.req == nil {
		return false
	}
	switch tab {
	case tabParams:
		return len(m.req.Params) > 0
	case tabHeaders:
		return len(m.req.Headers) > 0
	case tabBody:
		return m.req.Body.Content != ""
	case tabAuth:
		return m.req.Auth.Type != core.AuthNone
	case tabInfo:
		return m.req.Description != ""
	}
	return false
}

func (m Request) renderField(b *strings.Builder, row int, label, value string) {
	selected := m.selectedRow(row)
	line := fmt.Sprintf("%-7s %s", label, value)
	if selected {
		fmt.Fprintf(b, "%s\n", m.theme.Selected.Render(" "+line+" "))
		return
	}
	fmt.Fprintf(b, " %s %s\n", m.theme.FieldLabel.Render(fmt.Sprintf("%-7s", label)), value)
}

// renderURLLine draws the method and URL above the tabs, with {{vars}} as
// chips and the send hint on the right.
func (m Request) renderURLLine(b *strings.Builder) {
	method := m.theme.MethodPill(m.req.Method)
	if m.selectedRow(0) {
		method += m.theme.PaneActive.Render(" ▾")
	} else {
		method += m.theme.Muted.Render(" ▾")
	}

	if m.insert && m.onURLRow() {
		fmt.Fprintf(b, "%s  %s\n", method, m.urlInput.View())
		m.renderSuggestions(b)
		return
	}

	url := m.renderChips(m.req.URL)
	if m.req.URL == "" {
		url = m.theme.Muted.Render("(no url — press enter)")
	}
	if m.selectedRow(1) {
		url = m.theme.Selected.Render(" " + m.req.URL + " ")
	}

	line := SplitRow(method+"  "+url, m.sendHint(), m.width-4)
	fmt.Fprintf(b, "%s\n", line)
}

// urlToken splits a URL into variables, path params, and plain text so each
// reads in its own color.
var urlToken = regexp.MustCompile(`\{\{[^}]*\}\}|\{[^}/]*\}|:[A-Za-z_][A-Za-z0-9_]*`)

func (m Request) renderChips(s string) string {
	if s == "" {
		return ""
	}
	text := lipgloss.NewStyle().Foreground(m.theme.Text)

	var out strings.Builder
	last := 0
	for _, loc := range urlToken.FindAllStringIndex(s, -1) {
		out.WriteString(text.Render(s[last:loc[0]]))
		token := s[loc[0]:loc[1]]
		if strings.HasPrefix(token, "{{") {
			out.WriteString(m.theme.VarChip.Render(token))
		} else {
			out.WriteString(m.theme.PathChip.Render(token))
		}
		last = loc[1]
	}
	out.WriteString(text.Render(s[last:]))
	return out.String()
}

func (m Request) sendHint() string {
	label := "Send"
	if icon := m.theme.Icons.Send; icon != "" {
		label = icon + " Send"
	}
	return lipgloss.NewStyle().Foreground(m.theme.Flavor.Green()).Bold(true).Render(label)
}

// selectedRow reports whether an absolute row is the highlighted one.
func (m Request) selectedRow(row int) bool {
	return m.focused && !m.insert && m.row == row
}

func (m Request) renderInfoTab(b *strings.Builder) {
	fmt.Fprintf(b, " %s %s\n", m.theme.FieldLabel.Render(fmt.Sprintf("%-7s", "name")), m.req.Name)
	if m.insert {
		fmt.Fprintf(b, " %s %s\n", m.theme.FieldLabel.Render(fmt.Sprintf("%-7s", "desc")), m.infoInput.View())
		return
	}
	desc := m.req.Description
	if desc == "" {
		desc = m.theme.Muted.Render("(none)")
	}
	m.renderField(b, urlRows, "desc", desc)
	fmt.Fprintf(b, "\n %s %s\n", m.theme.FieldLabel.Render(fmt.Sprintf("%-7s", "path")),
		m.theme.Muted.Render(strings.Join(append([]string{m.collection}, m.path...), "/")))
}

func (m Request) renderSuggestions(b *strings.Builder) {
	if line := m.suggestionLine(); line != "" {
		fmt.Fprintf(b, "%s\n", line)
	}
}

func (m Request) renderKVTab(b *strings.Builder, kvs []core.KV, emptyHint string) {
	if len(kvs) == 0 {
		fmt.Fprintf(b, " %s\n", m.theme.Muted.Render("no "+emptyHint))
		return
	}
	for i, kv := range kvs {
		if m.insert && i == m.tabRow() {
			fmt.Fprintf(b, " %s = %s\n", m.kvName.View(), m.kvValue.View())
			m.renderSuggestions(b)
			continue
		}
		selected := m.selectedRow(urlRows + i)
		line := fmt.Sprintf("%s = %s", kv.Name, kv.Value)
		if selected {
			fmt.Fprintf(b, "%s\n", m.theme.Selected.Render(" "+line+" "))
		} else {
			fmt.Fprintf(b, " %s\n", line)
		}
	}
}

func (m Request) renderBodyTab(b *strings.Builder) {
	bodyType := m.req.Body.Type
	if bodyType == "" {
		bodyType = "none"
	}
	typeBar := m.theme.Bar.Width(m.width - 4).
		Render(" " + strings.ToUpper(bodyType) + "  ▾")
	if m.selectedRow(urlRows) {
		typeBar = m.theme.Selected.Width(m.width - 4).
			Render(" " + strings.ToUpper(bodyType) + "  ▾")
	}
	fmt.Fprintf(b, "%s\n\n", typeBar)
	if m.insert {
		fmt.Fprintf(b, "%s\n", m.bodyArea.View())
		return
	}
	if m.req.Body.Content == "" {
		m.renderField(b, urlRows+1, "content", m.theme.Muted.Render("(empty — enter to edit)"))
		return
	}
	if m.selectedRow(urlRows + 1) {
		fmt.Fprintf(b, "%s\n", m.theme.Selected.Render(" content "))
	} else {
		fmt.Fprintf(b, "\n")
	}

	lines := strings.Split(m.bodyPreview(), "\n")
	width := len(fmt.Sprint(len(lines)))
	for i, line := range lines {
		if m.lineNumbers {
			fmt.Fprintf(b, " %s  %s\n",
				m.theme.Gutter.Render(fmt.Sprintf("%*d", width, i+1)), line)
		} else {
			fmt.Fprintf(b, "   %s\n", line)
		}
	}
}

// bodyPreview syntax-highlights JSON bodies in normal mode, capped by size
// so huge bodies never block rendering.
func (m Request) bodyPreview() string {
	content := m.req.Body.Content
	looksJSON := m.req.Body.Type == "json" ||
		strings.HasPrefix(strings.TrimSpace(content), "{") ||
		strings.HasPrefix(strings.TrimSpace(content), "[")
	if !looksJSON || len(content) > highlightLimit {
		return content
	}
	var highlighted bytes.Buffer
	if err := quick.Highlight(&highlighted, content, "json", "terminal16m", m.theme.ChromaStyle()); err != nil {
		return content
	}
	return strings.TrimSuffix(highlighted.String(), "\n")
}

func (m Request) renderAuthTab(b *strings.Builder) {
	authType := m.req.Auth.Type
	if authType == core.AuthNone {
		authType = "none"
	}
	m.renderField(b, urlRows, "type", authType)

	switch m.req.Auth.Type {
	case core.AuthBearer:
		if m.insert {
			fmt.Fprintf(b, " %s %s\n", m.theme.FieldLabel.Render("token  "), m.authInput.View())
			m.renderSuggestions(b)
		} else {
			m.renderField(b, urlRows+1, "token", m.req.Auth.Token)
		}
	case core.AuthBasic:
		if m.insert && m.tabRow() == 1 {
			fmt.Fprintf(b, " %s %s\n", m.theme.FieldLabel.Render("user   "), m.authInput.View())
			m.renderSuggestions(b)
		} else {
			m.renderField(b, urlRows+1, "user", m.req.Auth.User)
		}
		if m.insert && m.tabRow() == 2 {
			fmt.Fprintf(b, " %s %s\n", m.theme.FieldLabel.Render("pass   "), m.authInput.View())
			m.renderSuggestions(b)
		} else {
			m.renderField(b, urlRows+2, "pass", strings.Repeat("•", len(m.req.Auth.Pass)))
		}
	}
}
