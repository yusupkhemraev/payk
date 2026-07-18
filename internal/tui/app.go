// Package tui contains the root Bubble Tea model. The root model only routes
// messages, tracks focus, and composes panel views; panel behavior lives in
// the panels package.
package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yusupkhemraev/payk/internal/tui/keymap"
	"github.com/yusupkhemraev/payk/internal/tui/panels"
	"github.com/yusupkhemraev/payk/internal/tui/theme"
)

// Config is the runtime configuration of the TUI.
type Config struct {
	// WorkspaceDir points at a .payk directory explicitly; empty means
	// discover one from the working directory.
	WorkspaceDir string
}

type pane int

const (
	paneCollections pane = iota
	paneRequest
	paneResponse
)

func (p pane) String() string {
	switch p {
	case paneCollections:
		return "collections"
	case paneRequest:
		return "request"
	case paneResponse:
		return "response"
	}
	return "unknown"
}

// Model is the root model composing the three panels and the status bar.
type Model struct {
	cfg   Config
	theme *theme.Theme
	keys  keymap.KeyMap

	width  int
	height int
	sizes  PanelSizes

	focus          pane
	lastMain       pane
	sidebarVisible bool
	showHelp       bool

	collections panels.Collections
	request     panels.Request
	response    panels.Response
}

// New builds the root model with the default theme.
func New(cfg Config) Model {
	t := theme.Default()
	keys := keymap.Default()
	m := Model{
		cfg:            cfg,
		theme:          t,
		keys:           keys,
		focus:          paneCollections,
		lastMain:       paneRequest,
		sidebarVisible: true,
		collections:    panels.NewCollections(t, keys),
		request:        panels.NewRequest(t),
		response:       panels.NewResponse(t),
	}
	m.applyFocus()
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return loadWorkspaceCmd(m.cfg)
}

// Update implements tea.Model; it only routes messages and tracks focus.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyLayout()
		return m, nil

	case workspaceLoadedMsg:
		m.collections.SetWorkspace(msg.collections, msg.workspace != nil, msg.err)
		return m, nil

	case panels.RequestSelectedMsg:
		m.request.SetRequest(msg.Request)
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		switch {
		case key.Matches(msg, m.keys.Help),
			key.Matches(msg, m.keys.Escape),
			key.Matches(msg, m.keys.Quit):
			m.showHelp = false
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.showHelp = true
		return m, nil

	case key.Matches(msg, m.keys.ToggleSidebar):
		m.sidebarVisible = !m.sidebarVisible
		m.applyLayout()
		return m, nil

	case key.Matches(msg, m.keys.FocusLeft):
		m.moveFocus(-1, false)
		return m, nil

	case key.Matches(msg, m.keys.FocusRight):
		m.moveFocus(1, false)
		return m, nil

	case key.Matches(msg, m.keys.NextPane):
		m.moveFocus(1, true)
		return m, nil

	case key.Matches(msg, m.keys.PrevPane):
		m.moveFocus(-1, true)
		return m, nil
	}

	return m.routeToFocused(msg)
}

func (m Model) routeToFocused(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focus {
	case paneCollections:
		m.collections, cmd = m.collections.Update(msg)
	case paneRequest:
		m.request, cmd = m.request.Update(msg)
	case paneResponse:
		m.response, cmd = m.response.Update(msg)
	}
	return m, cmd
}

// focusOrder lists the panes reachable at the current layout, left to right.
func (m Model) focusOrder() []pane {
	if m.sizes.Mode == ModeDouble && !m.sidebarVisible {
		return []pane{paneRequest, paneResponse}
	}
	return []pane{paneCollections, paneRequest, paneResponse}
}

func (m *Model) moveFocus(delta int, wrap bool) {
	order := m.focusOrder()
	idx := 0
	for i, p := range order {
		if p == m.focus {
			idx = i
			break
		}
	}
	idx += delta
	if wrap {
		idx = (idx + len(order)) % len(order)
	} else {
		idx = clamp(idx, 0, len(order)-1)
	}
	m.focus = order[idx]
	if m.focus == paneRequest || m.focus == paneResponse {
		m.lastMain = m.focus
	}
	m.applyFocus()
}

func (m *Model) applyFocus() {
	m.collections.SetFocused(m.focus == paneCollections)
	m.request.SetFocused(m.focus == paneRequest)
	m.response.SetFocused(m.focus == paneResponse)
}

func (m *Model) applyLayout() {
	m.sizes = layout(m.width, m.height, m.sidebarVisible)
	m.collections.SetSize(m.sizes.Collections.Width, m.sizes.Collections.Height)
	m.request.SetSize(m.sizes.Request.Width, m.sizes.Request.Height)
	m.response.SetSize(m.sizes.Response.Width, m.sizes.Response.Height)

	if m.sizes.Mode == ModeDouble && !m.sidebarVisible && m.focus == paneCollections {
		m.focus = m.lastMain
	}
	m.applyFocus()
}

// View implements tea.Model.
func (m Model) View() tea.View {
	var content string
	if m.width > 0 && m.height > 0 {
		if m.showHelp {
			content = m.helpView()
		} else {
			content = m.panesView()
		}
		content = lipgloss.JoinVertical(lipgloss.Left, content, m.statusView())
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.BackgroundColor = m.theme.Base
	return v
}

func (m Model) panesView() string {
	switch m.sizes.Mode {
	case ModeTriple:
		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.collections.View(),
			m.request.View(),
			m.response.View(),
		)

	case ModeDouble:
		main := m.request.View()
		if m.lastMain == paneResponse {
			main = m.response.View()
		}
		if m.sidebarVisible {
			return lipgloss.JoinHorizontal(lipgloss.Top, m.collections.View(), main)
		}
		return lipgloss.JoinHorizontal(lipgloss.Top, m.request.View(), m.response.View())

	default:
		switch m.focus {
		case paneRequest:
			return m.request.View()
		case paneResponse:
			return m.response.View()
		default:
			return m.collections.View()
		}
	}
}

func (m Model) helpView() string {
	var columns []string
	for _, group := range m.keys.FullHelp() {
		var lines []string
		for _, binding := range group {
			if !binding.Enabled() {
				continue
			}
			lines = append(lines,
				m.theme.HelpKey.Render(padRight(binding.Help().Key, 10))+
					m.theme.HelpDesc.Render(binding.Help().Desc))
		}
		if len(lines) > 0 {
			columns = append(columns, strings.Join(lines, "\n"))
		}
	}

	box := m.theme.HelpBox.Render(
		m.theme.HelpTitle.Render("payk — keybindings") + "\n" +
			lipgloss.JoinHorizontal(lipgloss.Top, joinWithGap(columns, 4)...),
	)

	return lipgloss.Place(m.width, m.height-statusBarHeight, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) statusView() string {
	badge := m.theme.StatusBadge.Render("payk")
	focus := m.theme.StatusFocus.Render(m.focus.String())
	hint := m.theme.StatusHint.Render("? help · tab pane · q quit")

	bar := lipgloss.JoinHorizontal(lipgloss.Top, badge, focus, hint)
	return m.theme.StatusBar.Width(m.width).MaxWidth(m.width).Render(bar)
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s + " "
	}
	return s + strings.Repeat(" ", width-len(s))
}

func joinWithGap(columns []string, gap int) []string {
	spacer := strings.Repeat(" ", gap)
	out := make([]string, 0, len(columns)*2)
	for i, c := range columns {
		if i > 0 {
			out = append(out, spacer)
		}
		out = append(out, c)
	}
	return out
}
