package panels

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

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

type nodeKind int

const (
	nodeCollection nodeKind = iota
	nodeFolder
	nodeRequest
)

// node is one tree entry; folders and collections hold children.
type node struct {
	kind     nodeKind
	name     string
	depth    int
	expanded bool
	children []*node

	request    *core.Request
	collection string
	path       []string
}

// Collections is the left panel: the collections/requests tree.
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
}

// NewCollections builds the collections panel in its loading state.
func NewCollections(t *theme.Theme, keys keymap.KeyMap) Collections {
	return Collections{theme: t, keys: keys, loading: true}
}

// SetWorkspace replaces the tree content after the workspace load finishes.
func (m *Collections) SetWorkspace(collections []*core.Collection, hasWorkspace bool, err error) {
	m.loading = false
	m.loadErr = err
	m.hasWorkspace = hasWorkspace
	m.roots = buildNodes(collections)
	m.cursor = 0
	m.scroll = 0
	m.refreshVisible()
}

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
			expanded:   true,
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

// SetFocused toggles keyboard focus for this panel.
func (m *Collections) SetFocused(focused bool) {
	m.focused = focused
	if !focused {
		m.pendingG = false
	}
}

// rowCount is the number of tree rows that fit inside the frame.
func (m Collections) rowCount() int {
	return max(m.height-3, 1)
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

// Update handles navigation keys while the panel is focused.
func (m Collections) Update(msg tea.Msg) (Collections, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok || len(m.visible) == 0 {
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

	case key.Matches(keyMsg, m.keys.Select):
		return m.openCurrent()
	}
	return m, nil
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

// View renders the panel at its current size.
func (m Collections) View() string {
	return frame(m.theme, "Collections", m.focused, m.width, m.height, m.body())
}

func (m Collections) body() string {
	switch {
	case m.loading:
		return m.theme.Muted.Render(" loading…")
	case m.loadErr != nil:
		return m.theme.Muted.Render(" error: " + m.loadErr.Error())
	case !m.hasWorkspace:
		return m.theme.Muted.Render(" no workspace found\n\n create .payk/collections/\n in your project")
	case len(m.visible) == 0:
		return m.theme.Muted.Render(" workspace is empty\n\n add YAML files under\n .payk/collections/")
	}

	rows := m.rowCount()
	end := min(m.scroll+rows, len(m.visible))

	var b strings.Builder
	for i := m.scroll; i < end; i++ {
		if i > m.scroll {
			b.WriteString("\n")
		}
		b.WriteString(m.renderRow(m.visible[i], i == m.cursor && m.focused))
	}
	return b.String()
}

func (m Collections) renderRow(n *node, selected bool) string {
	indent := strings.Repeat("  ", n.depth)

	// The selected row gets a single background style; nested foreground
	// styles would reset it mid-row, so it is built from plain text.
	if selected {
		return m.theme.Selected.Render(" " + indent + n.plainLabel() + " ")
	}

	switch n.kind {
	case nodeRequest:
		badge := m.theme.Method(n.request.Method).Render(fmt.Sprintf("%-6s", n.request.Method))
		return " " + indent + badge + " " + n.name
	default:
		return " " + m.theme.Muted.Render(indent+n.arrow()) + " " + n.name
	}
}

func (n *node) arrow() string {
	if n.expanded {
		return "▾"
	}
	return "▸"
}

func (n *node) plainLabel() string {
	if n.kind == nodeRequest {
		return fmt.Sprintf("%-6s %s", n.request.Method, n.name)
	}
	return n.arrow() + " " + n.name
}
