package panels

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yusupkhemraev/payk/internal/core"
	"github.com/yusupkhemraev/payk/internal/tui/keymap"
	"github.com/yusupkhemraev/payk/internal/tui/theme"
)

// RequestSelectedMsg is emitted when the user opens a request in the tree.
// The root model routes it to the request editor panel.
type RequestSelectedMsg struct {
	Collection string
	Path       []string
	Request    *core.Request
}

// RenameRequestedMsg is emitted when the user confirms an inline rename; the
// root model performs the storage operation and reloads the tree.
type RenameRequestedMsg struct {
	Collection string
	Path       []string
	// Request is nil when a folder or collection is being renamed.
	Request *core.Request
	NewName string
}

// DeleteRequestedMsg is emitted when the user confirms deleting a tree node;
// the root model removes it from disk and reloads the tree.
type DeleteRequestedMsg struct {
	Collection string
	Path       []string
	// Request is nil when a folder or collection is being deleted.
	Request *core.Request
	Label   string
}

// CreateRequestedMsg asks the root model to create a new request in the
// given collection/folder and open it in the editor.
type CreateRequestedMsg struct {
	Collection string
	Path       []string
	Name       string
}

type nodeKind int

const (
	nodeCollection nodeKind = iota
	nodeFolder
	nodeRequest
)

type node struct {
	kind     nodeKind
	name     string
	depth    int
	expanded bool
	parent   *node
	children []*node

	request    *core.Request
	collection string
	path       []string
}

type Collections struct {
	theme *theme.Theme
	keys  keymap.KeyMap

	width   int
	height  int
	focused bool

	loading      bool
	loadErr      error
	hasWorkspace bool

	roots    []*node
	visible  []*node
	cursor   int
	scroll   int
	pendingG bool

	searching   bool
	searchInput textinput.Model

	renaming    bool
	renameInput textinput.Model

	adding   bool
	addInput textinput.Model

	// pendingDelete awaits y/n confirmation.
	pendingDelete *node

	// statuses maps a request key to its last response label; showStatus
	// mirrors the config toggle.
	statuses   map[string]string
	showStatus bool
}

// SetStatuses supplies the last response label per request key.
func (m *Collections) SetStatuses(statuses map[string]string, show bool) {
	m.statuses = statuses
	m.showStatus = show
}

// statusFor returns the cached response label for a request node.
func (m Collections) statusFor(n *node) string {
	if !m.showStatus || n.kind != nodeRequest || m.statuses == nil {
		return ""
	}
	parts := append([]string{n.collection}, n.path...)
	parts = append(parts, n.name)
	return m.statuses[strings.Join(parts, "/")]
}

// statusStyle colors a cached label by its class.
func (m Collections) statusStyle(label string) lipgloss.Style {
	switch {
	case label == "ERR":
		return m.theme.Status(500)
	case strings.HasPrefix(label, "2"):
		return m.theme.Status(200)
	case strings.HasPrefix(label, "3"):
		return m.theme.Status(300)
	case strings.HasPrefix(label, "4"):
		return m.theme.Status(400)
	case strings.HasPrefix(label, "5"):
		return m.theme.Status(500)
	}
	return m.theme.Muted
}

func NewCollections(t *theme.Theme, keys keymap.KeyMap) Collections {
	search := textinput.New()
	search.Prompt = "/"
	rename := textinput.New()
	rename.Prompt = "rename: "
	add := textinput.New()
	add.Prompt = "new request: "
	return Collections{
		theme: t, keys: keys, loading: true,
		searchInput: search, renameInput: rename, addInput: add,
	}
}

// Capturing reports whether an inline input (search, rename, add, delete
// confirmation) captures keystrokes; the root model must not treat keys as
// global shortcuts then.
func (m *Collections) Capturing() bool {
	return m.searching || m.renaming || m.adding || m.pendingDelete != nil
}

// SetTheme swaps the theme after a config reload.
func (m *Collections) SetTheme(t *theme.Theme) {
	m.theme = t
}

func (m *Collections) SetWorkspace(collections []*core.Collection, hasWorkspace bool, err error) {
	m.loading = false
	m.loadErr = err
	m.hasWorkspace = hasWorkspace
	m.roots = buildNodes(collections)
	m.cursor = 0
	m.scroll = 0
	m.refreshVisible()
}

// buildNodes expands collections but keeps folders collapsed, so a fresh
// tree starts compact.
func buildNodes(collections []*core.Collection) []*node {
	var roots []*node
	for _, c := range collections {
		root := &node{kind: nodeCollection, name: c.Name, expanded: true, collection: c.Name}
		root.children = buildFolderChildren(c.Folders, c.Requests, root, nil)
		roots = append(roots, root)
	}
	return roots
}

func buildFolderChildren(folders []*core.Folder, requests []*core.Request, parent *node, path []string) []*node {
	var children []*node
	for _, f := range folders {
		fn := &node{
			kind:       nodeFolder,
			name:       f.Name,
			depth:      parent.depth + 1,
			expanded:   false,
			parent:     parent,
			collection: parent.collection,
			path:       append(append([]string{}, path...), f.Name),
		}
		fn.children = buildFolderChildren(f.Folders, f.Requests, fn, fn.path)
		children = append(children, fn)
	}
	for _, r := range requests {
		children = append(children, &node{
			kind:       nodeRequest,
			name:       r.Name,
			depth:      parent.depth + 1,
			parent:     parent,
			request:    r,
			collection: parent.collection,
			path:       path,
		})
	}
	return children
}

func (m *Collections) refreshVisible() {
	m.visible = m.visible[:0]
	for _, root := range m.roots {
		m.appendVisible(root)
	}
	if m.cursor > len(m.visible)-1 {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureCursorVisible()
}

func (m *Collections) appendVisible(n *node) {
	m.visible = append(m.visible, n)
	if n.expanded {
		for _, child := range n.children {
			m.appendVisible(child)
		}
	}
}

// SetSize sets the outer box size, borders included.
func (m *Collections) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.ensureCursorVisible()
}

func (m *Collections) SetFocused(focused bool) {
	m.focused = focused
	if !focused {
		m.pendingG = false
		if m.searching {
			m.searching = false
			m.searchInput.Blur()
			m.cursor = 0
			m.refreshVisible()
		}
		if m.renaming {
			m.renaming = false
			m.renameInput.Blur()
		}
		if m.adding {
			m.adding = false
			m.addInput.Blur()
		}
	}
}

// rowCount is the number of tree rows that fit inside the frame; the search
// or rename line takes one row while active.
func (m Collections) rowCount() int {
	rows := m.height - 4
	if m.Capturing() {
		rows--
	}
	return max(rows, 1)
}

func (m *Collections) ensureCursorVisible() {
	rows := m.rowCount()
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+rows {
		m.scroll = m.cursor - rows + 1
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m Collections) Update(msg tea.Msg) (Collections, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	if m.searching {
		return m.updateSearch(keyMsg)
	}
	if m.renaming {
		return m.updateRename(keyMsg)
	}
	if m.adding {
		return m.updateAdd(keyMsg)
	}
	if m.pendingDelete != nil {
		return m.updateDeleteConfirm(keyMsg)
	}

	// Adding works even before any workspace exists: the root model
	// creates ./.payk and a default collection on the fly.
	if key.Matches(keyMsg, m.keys.AddRow) && !m.loading {
		m.adding = true
		m.addInput.SetValue("")
		return m, m.addInput.Focus()
	}
	if len(m.visible) == 0 && len(m.roots) == 0 {
		return m, nil
	}

	// gg chord: first g arms the chord, second g jumps to top.
	if m.pendingG {
		m.pendingG = false
		if keyMsg.String() == "g" {
			m.cursor = 0
			m.ensureCursorVisible()
		}
		return m, nil
	}

	switch {
	case key.Matches(keyMsg, m.keys.Search):
		m.searching = true
		m.searchInput.SetValue("")
		m.applyFilter()
		return m, m.searchInput.Focus()

	case key.Matches(keyMsg, m.keys.Down):
		if m.cursor < len(m.visible)-1 {
			m.cursor++
		}
		m.ensureCursorVisible()

	case key.Matches(keyMsg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		m.ensureCursorVisible()

	case key.Matches(keyMsg, m.keys.Bottom):
		m.cursor = len(m.visible) - 1
		m.ensureCursorVisible()

	case key.Matches(keyMsg, m.keys.Top):
		m.pendingG = true

	case key.Matches(keyMsg, m.keys.Rename):
		if m.cursor < len(m.visible) {
			m.renaming = true
			m.renameInput.SetValue(m.visible[m.cursor].name)
			m.renameInput.CursorEnd()
			return m, m.renameInput.Focus()
		}

	case key.Matches(keyMsg, m.keys.DeleteRow):
		if m.cursor < len(m.visible) {
			m.pendingDelete = m.visible[m.cursor]
		}

	case key.Matches(keyMsg, m.keys.Select):
		return m.openCurrent()
	}
	return m, nil
}

func (m Collections) updateAdd(msg tea.KeyPressMsg) (Collections, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.adding = false
		m.addInput.Blur()
		return m, nil

	case msg.Code == tea.KeyEnter:
		m.adding = false
		m.addInput.Blur()
		name := strings.TrimSpace(m.addInput.Value())
		if name == "" {
			return m, nil
		}
		create := CreateRequestedMsg{Collection: "api", Name: name}
		if m.cursor < len(m.visible) {
			n := m.visible[m.cursor]
			create.Collection = n.collection
			create.Path = n.path
		}
		return m, func() tea.Msg { return create }
	}

	var cmd tea.Cmd
	m.addInput, cmd = m.addInput.Update(msg)
	return m, cmd
}

// SelectRequest finds a request in the tree, reveals it, and returns it so
// the root model can open it in the editor.
func (m *Collections) SelectRequest(collection string, path []string, name string) *core.Request {
	var target *node
	var walk func(n *node)
	walk = func(n *node) {
		if target != nil {
			return
		}
		if n.kind == nodeRequest && n.collection == collection &&
			n.name == name && slices.Equal(n.path, path) {
			target = n
			return
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	for _, root := range m.roots {
		walk(root)
	}
	if target == nil {
		return nil
	}
	m.jumpTo(target)
	return target.request
}

func (m Collections) updateDeleteConfirm(msg tea.KeyPressMsg) (Collections, tea.Cmd) {
	n := m.pendingDelete
	m.pendingDelete = nil
	if msg.String() != "y" && msg.String() != "Y" {
		return m, nil
	}
	del := DeleteRequestedMsg{
		Collection: n.collection,
		Path:       n.path,
		Request:    n.request,
		Label:      n.name,
	}
	return m, func() tea.Msg { return del }
}

func (m Collections) updateRename(msg tea.KeyPressMsg) (Collections, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.renaming = false
		m.renameInput.Blur()
		return m, nil

	case msg.Code == tea.KeyEnter:
		m.renaming = false
		m.renameInput.Blur()
		newName := strings.TrimSpace(m.renameInput.Value())
		if m.cursor >= len(m.visible) || newName == "" {
			return m, nil
		}
		n := m.visible[m.cursor]
		if newName == n.name {
			return m, nil
		}
		rename := RenameRequestedMsg{
			Collection: n.collection,
			Path:       n.path,
			Request:    n.request,
			NewName:    newName,
		}
		return m, func() tea.Msg { return rename }
	}

	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
}

func (m Collections) updateSearch(msg tea.KeyPressMsg) (Collections, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.searching = false
		m.searchInput.Blur()
		m.cursor = 0
		m.refreshVisible()
		return m, nil

	case msg.Code == tea.KeyEnter:
		var target *node
		if m.cursor < len(m.visible) {
			target = m.visible[m.cursor]
		}
		m.searching = false
		m.searchInput.Blur()
		m.jumpTo(target)
		return m, nil

	case msg.Code == tea.KeyDown || msg.String() == "ctrl+n":
		if m.cursor < len(m.visible)-1 {
			m.cursor++
		}
		m.ensureCursorVisible()
		return m, nil

	case msg.Code == tea.KeyUp || msg.String() == "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
		}
		m.ensureCursorVisible()
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m *Collections) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))
	if query == "" {
		m.refreshVisible()
		return
	}

	m.visible = m.visible[:0]
	var walk func(n *node)
	walk = func(n *node) {
		if n.matches(query) {
			m.visible = append(m.visible, n)
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	for _, root := range m.roots {
		walk(root)
	}
	m.cursor = 0
	m.scroll = 0
}

// matches checks the query against name, method, URL, and tree path.
func (n *node) matches(query string) bool {
	if strings.Contains(strings.ToLower(n.name), query) {
		return true
	}
	if n.request != nil {
		if strings.Contains(strings.ToLower(n.request.Method), query) ||
			strings.Contains(strings.ToLower(n.request.URL), query) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(n.pathLabel()), query)
}

func (n *node) pathLabel() string {
	parts := append([]string{n.collection}, n.path...)
	return strings.Join(parts, "/")
}

// jumpTo restores the full tree with target selected, expanding its
// ancestors so it is visible.
func (m *Collections) jumpTo(target *node) {
	if target != nil {
		for p := target.parent; p != nil; p = p.parent {
			p.expanded = true
		}
	}
	m.refreshVisible()
	if target == nil {
		return
	}
	for i, n := range m.visible {
		if n == target {
			m.cursor = i
			break
		}
	}
	m.ensureCursorVisible()
}

func (m Collections) openCurrent() (Collections, tea.Cmd) {
	n := m.visible[m.cursor]
	if n.kind == nodeRequest {
		msg := RequestSelectedMsg{Collection: n.collection, Path: n.path, Request: n.request}
		return m, func() tea.Msg { return msg }
	}
	n.expanded = !n.expanded
	m.refreshVisible()
	return m, nil
}

func (n *node) requestCount() int {
	if n.kind == nodeRequest {
		return 1
	}
	total := 0
	for _, child := range n.children {
		total += child.requestCount()
	}
	return total
}

func (m Collections) View() string {
	meta := ""
	if total := m.totalRequests(); total > 0 {
		meta = fmt.Sprintf("· %d", total)
	}
	return frame(m.theme, "COLLECTIONS", meta, m.focused, m.width, m.height, m.body())
}

func (m Collections) totalRequests() int {
	total := 0
	for _, root := range m.roots {
		total += root.requestCount()
	}
	return total
}

func (m Collections) body() string {
	switch {
	case m.loading:
		return m.theme.Muted.Render(" loading…")
	case m.loadErr != nil:
		return m.theme.Muted.Render(" error: " + m.loadErr.Error())
	case (!m.hasWorkspace || len(m.visible) == 0) && !m.searching && !m.adding:
		return m.theme.Muted.Render(" no requests yet\n\n" +
			" a — create a request\n" +
			" :import <spec|url|dir>\n" +
			" or paste a curl command")
	}

	var b strings.Builder
	if m.searching {
		fmt.Fprintf(&b, " %s %s\n", m.searchInput.View(),
			m.theme.Muted.Render(fmt.Sprintf("%d", len(m.visible))))
	}
	if m.renaming {
		fmt.Fprintf(&b, " %s\n", m.renameInput.View())
	}
	if m.adding {
		fmt.Fprintf(&b, " %s\n", m.addInput.View())
	}
	if m.pendingDelete != nil {
		fmt.Fprintf(&b, " %s\n", m.theme.Status(400).Render(
			fmt.Sprintf("delete %q? (y/n)", m.pendingDelete.name)))
	}

	rows := m.rowCount()
	end := min(m.scroll+rows, len(m.visible))
	for i := m.scroll; i < end; i++ {
		if i > m.scroll {
			b.WriteString("\n")
		}
		b.WriteString(m.renderRow(m.visible[i], i == m.cursor && m.focused))
	}
	if m.searching && len(m.visible) == 0 {
		b.WriteString(m.theme.Muted.Render(" no matches"))
	}
	return b.String()
}

func (m Collections) renderRow(n *node, selected bool) string {
	indent := strings.Repeat("  ", n.depth)
	// Search results render flat, with the tree path as context instead of
	// indentation.
	var context string
	if m.searching {
		indent = ""
		context = "  " + n.pathLabel()
	}

	inner := m.width - 2
	status := m.statusFor(n)

	// The selected row gets a single background style stretched across the
	// panel; nested foreground styles would reset it mid-row, so it is
	// built from plain text.
	if selected {
		label := " " + indent + n.plainLabel(m.theme.Icons) + context
		if status != "" {
			return m.theme.Selected.Render(SplitRow(label, status+" ", inner))
		}
		return m.theme.Selected.Render(Pad(label, inner))
	}

	styledContext := ""
	if context != "" {
		styledContext = m.theme.Muted.Render(context)
	}
	styledStatus := ""
	if status != "" {
		styledStatus = m.statusStyle(status).Render(status) + " "
	}

	switch n.kind {
	case nodeRequest:
		badge := m.theme.Method(n.request.Method).Render(fmt.Sprintf("%-6s", n.request.Method))
		row := " " + indent + badge + " " + n.name + styledContext
		if styledStatus != "" {
			return SplitRow(row, styledStatus, inner)
		}
		return row
	case nodeCollection:
		count := m.theme.TreeCount.Render(fmt.Sprintf(" · %d", n.requestCount()))
		return " " + m.theme.Muted.Render(n.folderIcon(m.theme.Icons)) + " " +
			m.theme.TreeCollection.Render(n.name) + count + styledContext
	default:
		count := m.theme.TreeCount.Render(fmt.Sprintf(" · %d", n.requestCount()))
		return " " + m.theme.Muted.Render(indent+n.folderIcon(m.theme.Icons)) + " " +
			m.theme.TreeFolder.Render(n.name) + count + styledContext
	}
}

// folderIcon picks the open/closed glyph for a collection or folder.
func (n *node) folderIcon(icons theme.Icons) string {
	if n.expanded {
		return icons.FolderOpen
	}
	return icons.Folder
}

func (n *node) plainLabel(icons theme.Icons) string {
	if n.kind == nodeRequest {
		return fmt.Sprintf("%-6s %s", n.request.Method, n.name)
	}
	return fmt.Sprintf("%s %s · %d", n.folderIcon(icons), n.name, n.requestCount())
}
