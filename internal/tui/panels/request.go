package panels

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/core"
	"github.com/yusupkhemraev/payk/internal/tui/keymap"
	"github.com/yusupkhemraev/payk/internal/tui/theme"
)

type editorTab int

const (
	tabURL editorTab = iota
	tabParams
	tabHeaders
	tabBody
	tabAuth
)

var editorTabNames = []string{"URL", "Params", "Headers", "Body", "Auth"}

var methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

var bodyTypes = []string{"json", "text", "form"}

var authTypes = []string{core.AuthNone, core.AuthBearer, core.AuthBasic}

// Request is the center panel: a tabbed request editor with vim-style
// normal/insert modes. Edits mutate the in-memory request; persisting to
// disk arrives with the command line (:w) in a later milestone.
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

	// envVars/osEnv feed {{var}} completion in insert mode.
	envVars []string
	osEnv   []string
	suggest *suggestState
}

// NewRequest builds the request editor panel.
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

	body := textarea.New()

	return Request{
		theme:     t,
		keys:      keys,
		urlInput:  url,
		kvName:    name,
		kvValue:   value,
		bodyArea:  body,
		authInput: auth,
	}
}

// SetRequest replaces the request loaded in the editor.
func (m *Request) SetRequest(req *core.Request) {
	m.req = req
	m.tab = tabURL
	m.row = 0
	m.insert = false
	m.suggest = nil
}

// SetEnvironment provides variable names for {{var}} completion: vars from
// the active environment and names from the process environment.
func (m *Request) SetEnvironment(envVars, osEnv []string) {
	m.envVars = envVars
	m.osEnv = osEnv
}

// CurrentRequest returns the request loaded in the editor, or nil.
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
	m.bodyArea.SetHeight(max(height-7, 3))
}

// SetFocused toggles keyboard focus for this panel.
func (m *Request) SetFocused(focused bool) {
	m.focused = focused
	if !focused && m.insert {
		m.commitInsert()
	}
}

// rows returns the number of selectable rows on the current tab.
func (m *Request) rows() int {
	switch m.tab {
	case tabURL:
		return 2 // method, url
	case tabParams:
		return len(m.req.Params)
	case tabHeaders:
		return len(m.req.Headers)
	case tabBody:
		return 2 // type, content
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

// Update handles keys while the panel is focused.
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
		m.row = 0

	case key.Matches(keyMsg, m.keys.TabPrev):
		m.tab = editorTab((int(m.tab) + len(editorTabNames) - 1) % len(editorTabNames))
		m.row = 0

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

	case key.Matches(keyMsg, m.keys.Select), key.Matches(keyMsg, m.keys.Insert):
		return m.activateRow()
	}
	return m, nil
}

func (m Request) addRow() (Request, tea.Cmd) {
	kv := core.KV{}
	if m.tab == tabParams {
		m.req.Params = append(m.req.Params, kv)
		m.row = len(m.req.Params) - 1
	} else {
		m.req.Headers = append(m.req.Headers, kv)
		m.row = len(m.req.Headers) - 1
	}
	return m.startKVInsert()
}

func (m *Request) deleteRow() {
	kvs := m.currentKVs()
	if m.row >= len(*kvs) {
		return
	}
	*kvs = append((*kvs)[:m.row], (*kvs)[m.row+1:]...)
	if m.row >= len(*kvs) && m.row > 0 {
		m.row--
	}
}

func (m *Request) currentKVs() *[]core.KV {
	if m.tab == tabParams {
		return &m.req.Params
	}
	return &m.req.Headers
}

// activateRow either cycles an enum row (method, body type, auth type) or
// enters insert mode on a text row.
func (m Request) activateRow() (Request, tea.Cmd) {
	switch m.tab {
	case tabURL:
		if m.row == 0 {
			m.req.Method = cycle(methods, m.req.Method)
			return m, nil
		}
		m.insert = true
		m.urlInput.SetValue(m.req.URL)
		m.urlInput.CursorEnd()
		m.refreshSuggestions()
		return m, m.urlInput.Focus()

	case tabParams, tabHeaders:
		if len(*m.currentKVs()) == 0 {
			return m.addRow()
		}
		return m.startKVInsert()

	case tabBody:
		if m.row == 0 {
			m.req.Body.Type = cycle(bodyTypes, m.req.Body.Type)
			return m, nil
		}
		m.insert = true
		m.bodyArea.SetValue(m.req.Body.Content)
		return m, m.bodyArea.Focus()

	case tabAuth:
		if m.row == 0 {
			m.req.Auth.Type = cycle(authTypes, m.req.Auth.Type)
			m.row = 0
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
	if m.row >= len(kvs) {
		return m, nil
	}
	m.insert = true
	m.kvField = 0
	m.kvName.SetValue(kvs[m.row].Name)
	m.kvName.CursorEnd()
	m.kvValue.SetValue(kvs[m.row].Value)
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

		case m.tab != tabBody && keyMsg.Code == tea.KeyEnter:
			m.commitInsert()
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.tab {
	case tabURL:
		m.urlInput, cmd = m.urlInput.Update(msg)
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
	switch m.tab {
	case tabURL:
		return &m.urlInput
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

// suggestionLine renders the completion candidates under the active input.
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

// commitInsert writes the active input back into the request and leaves
// insert mode.
func (m *Request) commitInsert() {
	switch m.tab {
	case tabURL:
		m.req.URL = strings.TrimSpace(m.urlInput.Value())
	case tabParams, tabHeaders:
		kvs := m.currentKVs()
		if m.row < len(*kvs) {
			kv := core.KV{
				Name:  strings.TrimSpace(m.kvName.Value()),
				Value: m.kvValue.Value(),
			}
			if kv.Name == "" && kv.Value == "" {
				*kvs = append((*kvs)[:m.row], (*kvs)[m.row+1:]...)
				if m.row > 0 {
					m.row--
				}
			} else {
				(*kvs)[m.row] = kv
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

	m.insert = false
	m.suggest = nil
	m.urlInput.Blur()
	m.kvName.Blur()
	m.kvValue.Blur()
	m.bodyArea.Blur()
	m.authInput.Blur()
}

func (m *Request) authValue() string {
	switch {
	case m.req.Auth.Type == core.AuthBearer:
		return m.req.Auth.Token
	case m.row == 1:
		return m.req.Auth.User
	default:
		return m.req.Auth.Pass
	}
}

func (m *Request) setAuthValue(v string) {
	switch {
	case m.req.Auth.Type == core.AuthBearer:
		m.req.Auth.Token = v
	case m.row == 1:
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

// View renders the panel at its current size.
func (m Request) View() string {
	return frame(m.theme, "Request", m.focused, m.width, m.height, m.body())
}

func (m Request) body() string {
	if m.req == nil {
		return m.tabBar() + "\n\n" + m.theme.Muted.Render(" select a request in collections (enter)")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", m.tabBar())

	switch m.tab {
	case tabURL:
		m.renderURLTab(&b)
	case tabParams:
		m.renderKVTab(&b, m.req.Params, "params — a to add")
	case tabHeaders:
		m.renderKVTab(&b, m.req.Headers, "headers — a to add")
	case tabBody:
		m.renderBodyTab(&b)
	case tabAuth:
		m.renderAuthTab(&b)
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
			return "tab name/value · {{ vars · esc done"
		}
		return "{{ vars · esc done"
	}
	return "[ ] tabs · i edit · space send"
}

func (m Request) tabBar() string {
	var parts []string
	for i, name := range editorTabNames {
		style := m.theme.TabInactive
		if editorTab(i) == m.tab {
			style = m.theme.TabActive
		}
		parts = append(parts, style.Render(name))
	}
	return " " + strings.Join(parts, m.theme.TabInactive.Render(" · "))
}

// renderRow draws one selectable line, highlighting it in normal mode.
func (m Request) renderField(b *strings.Builder, row int, label, value string) {
	selected := m.focused && !m.insert && m.row == row
	line := fmt.Sprintf("%-7s %s", label, value)
	if selected {
		fmt.Fprintf(b, "%s\n", m.theme.Selected.Render(" "+line+" "))
		return
	}
	fmt.Fprintf(b, " %s %s\n", m.theme.FieldLabel.Render(fmt.Sprintf("%-7s", label)), value)
}

func (m Request) renderURLTab(b *strings.Builder) {
	m.renderField(b, 0, "method", m.req.Method)
	if m.insert {
		fmt.Fprintf(b, " %s %s\n", m.theme.FieldLabel.Render("url    "), m.urlInput.View())
		m.renderSuggestions(b)
		return
	}
	m.renderField(b, 1, "url", m.req.URL)
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
		if m.insert && i == m.row {
			fmt.Fprintf(b, " %s = %s\n", m.kvName.View(), m.kvValue.View())
			m.renderSuggestions(b)
			continue
		}
		selected := m.focused && !m.insert && m.row == i
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
	m.renderField(b, 0, "type", bodyType)
	if m.insert {
		fmt.Fprintf(b, "%s\n", m.bodyArea.View())
		return
	}
	if m.req.Body.Content == "" {
		m.renderField(b, 1, "content", m.theme.Muted.Render("(empty — enter to edit)"))
		return
	}
	m.renderField(b, 1, "content", "")
	for line := range strings.SplitSeq(m.req.Body.Content, "\n") {
		fmt.Fprintf(b, "   %s\n", line)
	}
}

func (m Request) renderAuthTab(b *strings.Builder) {
	authType := m.req.Auth.Type
	if authType == core.AuthNone {
		authType = "none"
	}
	m.renderField(b, 0, "type", authType)

	switch m.req.Auth.Type {
	case core.AuthBearer:
		if m.insert {
			fmt.Fprintf(b, " %s %s\n", m.theme.FieldLabel.Render("token  "), m.authInput.View())
			m.renderSuggestions(b)
		} else {
			m.renderField(b, 1, "token", m.req.Auth.Token)
		}
	case core.AuthBasic:
		if m.insert && m.row == 1 {
			fmt.Fprintf(b, " %s %s\n", m.theme.FieldLabel.Render("user   "), m.authInput.View())
			m.renderSuggestions(b)
		} else {
			m.renderField(b, 1, "user", m.req.Auth.User)
		}
		if m.insert && m.row == 2 {
			fmt.Fprintf(b, " %s %s\n", m.theme.FieldLabel.Render("pass   "), m.authInput.View())
			m.renderSuggestions(b)
		} else {
			m.renderField(b, 2, "pass", strings.Repeat("•", len(m.req.Auth.Pass)))
		}
	}
}
