// Package tui contains the root Bubble Tea model. The root model only routes
// messages, tracks focus, and composes panel views; panel behavior lives in
// the panels package.
package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yusupkhemraev/payk/internal/core"
	"github.com/yusupkhemraev/payk/internal/importer"
	"github.com/yusupkhemraev/payk/internal/importer/curl"
	"github.com/yusupkhemraev/payk/internal/importer/fastapi"
	"github.com/yusupkhemraev/payk/internal/importer/openapi"
	"github.com/yusupkhemraev/payk/internal/storage"
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
	zoomed         bool
	sidebarDelta   int
	splitDelta     int

	sending    bool
	cancelSend context.CancelFunc

	workspace *storage.Workspace
	envs      *core.Environments
	// selCollection/selPath locate the loaded request in the tree for :w.
	selCollection string
	selPath       []string

	cmdline     cmdLine
	statusMsg   string
	statusIsErr bool
	helpScroll  int
	// messages keeps recent status texts in full; the m overlay shows them
	// since the status bar truncates.
	messages     []statusEntry
	showMessages bool

	importers []importer.Importer
	// pendingImport holds pasted text awaiting the y/n import prompt;
	// pendingImportKind names the importer that matched it.
	pendingImport     string
	pendingImportKind string

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
		envs:           &core.Environments{},
		cmdline:        newCmdLine(),
		importers:      []importer.Importer{curl.New(), openapi.New(), fastapi.New()},
		collections:    panels.NewCollections(t, keys),
		request:        panels.NewRequest(t, keys),
		response:       panels.NewResponse(t, keys),
	}
	m.applyFocus()
	return m
}

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
		m.workspace = msg.workspace
		m.envs = msg.environments
		m.collections.SetWorkspace(msg.collections, msg.workspace != nil, msg.err)
		m.syncEditorEnvironment()
		return m, nil

	case panels.RequestSelectedMsg:
		m.request.SetRequest(msg.Request)
		m.selCollection = msg.Collection
		m.selPath = msg.Path
		return m, nil

	case requestSavedMsg:
		if msg.err != nil {
			m.setStatus("write failed: "+msg.err.Error(), true)
		} else {
			m.setStatus("saved "+msg.name, false)
		}
		return m, nil

	case panels.StatusNote:
		m.setStatus(msg.Text, msg.IsErr)
		return m, nil

	case panels.RenameRequestedMsg:
		if m.workspace == nil {
			m.setStatus("no workspace to rename in", true)
			return m, nil
		}
		return m, renameCmd(m.workspace, m.cfg, msg)

	case panels.DeleteRequestedMsg:
		if m.workspace == nil {
			m.setStatus("no workspace to delete from", true)
			return m, nil
		}
		return m, deleteCmd(m.workspace, m.cfg, msg)

	case panels.CreateRequestedMsg:
		ws := m.workspace
		if ws == nil {
			wd, err := os.Getwd()
			if err != nil {
				m.setStatus("create failed: "+err.Error(), true)
				return m, nil
			}
			ws = &storage.Workspace{Dir: filepath.Join(wd, ".payk")}
		}
		return m, createRequestCmd(ws, m.cfg, msg)

	case requestCreatedMsg:
		m.workspace = msg.loaded.workspace
		m.envs = msg.loaded.environments
		m.collections.SetWorkspace(msg.loaded.collections, msg.loaded.workspace != nil, msg.loaded.err)
		m.syncEditorEnvironment()
		if req := m.collections.SelectRequest(msg.collection, msg.path, msg.name); req != nil {
			m.request.SetRequest(req)
			m.selCollection = msg.collection
			m.selPath = msg.path
			m.focus = paneRequest
			m.lastMain = paneRequest
			m.applyFocus()
		}
		m.setStatus("created "+msg.name+" — fill in the url and press space", false)
		return m, nil

	case panels.EditBodyRequestedMsg:
		return m, openEditorCmd(msg)

	case editorFinishedMsg:
		if msg.err != nil {
			m.setStatus("editor: "+msg.err.Error(), true)
			return m, nil
		}
		m.request.SetBodyContent(msg.content)
		m.setStatus("body updated from editor", false)
		return m, nil

	case tea.PasteMsg:
		// The open command line takes pastes (e.g. a spec URL for :import).
		if m.cmdline.active {
			return m, m.cmdline.update(msg)
		}
		// Pasted importable input (curl command, OpenAPI spec) offers an
		// inline import prompt; anything else goes to the focused panel
		// (e.g. an input in insert mode).
		if !m.request.Editing() {
			if imp := importer.Find(m.importers, msg.Content); imp != nil {
				m.pendingImport = msg.Content
				m.pendingImportKind = imp.Name()
				return m, nil
			}
		}
		return m.routeToFocused(msg)

	case importFinishedMsg:
		m.workspace = msg.workspace
		if msg.err != nil {
			m.setStatus("import failed: "+msg.err.Error(), true)
			return m, nil
		}
		m.collections.SetWorkspace(msg.collections, true, nil)
		if msg.environments != nil {
			m.envs = msg.environments
			m.syncEditorEnvironment()
		}
		// Each warning goes into the message log in full; the status bar
		// only shows the count.
		for _, warning := range msg.warnings {
			m.logMessage(warning, true)
		}
		status := "imported " + msg.imported
		if len(msg.warnings) > 0 {
			status += fmt.Sprintf(" · %d warnings — press m", len(msg.warnings))
		}
		m.setStatus(status, len(msg.warnings) > 0)
		return m, nil

	case environmentsSavedMsg:
		if msg.err != nil {
			m.setStatus("env save failed: "+msg.err.Error(), true)
		}
		return m, nil

	case responseReceivedMsg:
		m.sending = false
		m.cancelSend = nil
		if msg.err != nil {
			m.logMessage(msg.err.Error(), true)
		}
		m.response.SetResponse(msg.label, msg.resp, msg.err)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.response, cmd = m.response.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// handleImportPromptKey answers the inline "import as request?" prompt.
func (m Model) handleImportPromptKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	input := m.pendingImport
	switch msg.String() {
	case "y", "Y", "enter":
		m.pendingImport = ""
		m.pendingImportKind = ""
		return m, runImportCmd(m.importers, m.workspace, input)
	case "n", "N", "esc", "q":
		m.pendingImport = ""
		m.pendingImportKind = ""
		return m, nil
	}
	return m, nil
}

func pasteKindLabel(name string) string {
	switch name {
	case "curl":
		return "a curl command"
	case "openapi":
		return "an OpenAPI spec"
	case "fastapi":
		return "a FastAPI project"
	default:
		return "importable input"
	}
}

// handleCmdlineKey drives the ":" command input while it is open.
func (m Model) handleCmdlineKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.cmdline.close()
		return m, nil
	case msg.Code == tea.KeyEnter:
		line := m.cmdline.value()
		m.cmdline.close()
		return m.executeCommand(line)
	}
	return m, m.cmdline.update(msg)
}

// executeCommand runs a ":" command line.
func (m Model) executeCommand(line string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return m, nil
	}

	switch fields[0] {
	case "q", "quit":
		return m, tea.Quit

	case "send":
		return m.startSend()

	case "w", "write":
		req := m.request.CurrentRequest()
		switch {
		case req == nil:
			m.setStatus("nothing to write — no request open", true)
		case m.workspace == nil:
			m.setStatus("no workspace to write into", true)
		default:
			return m, saveRequestCmd(m.workspace, m.selCollection, m.selPath, req)
		}
		return m, nil

	case "env":
		if len(fields) < 2 {
			m.setStatus("usage: :env <name>", true)
			return m, nil
		}
		return m.switchEnvironment(fields[1])

	case "messages", "msgs":
		m.showMessages = true
		return m, nil

	case "import":
		if len(fields) < 2 {
			m.setStatus("usage: :import <file-or-url>", true)
			return m, nil
		}
		input := expandHome(strings.TrimSpace(strings.TrimPrefix(line, fields[0])))
		if importer.Find(m.importers, input) == nil {
			m.setStatus("no importer can handle: "+input, true)
			return m, nil
		}
		m.setStatus("importing "+input+"…", false)
		return m, runImportCmd(m.importers, m.workspace, input)

	default:
		m.setStatus("unknown command: "+fields[0], true)
		return m, nil
	}
}

func (m Model) switchEnvironment(name string) (tea.Model, tea.Cmd) {
	if m.envs.Get(name) == nil {
		m.setStatus("no such environment: "+name, true)
		return m, nil
	}
	m.envs.Active = name
	m.syncEditorEnvironment()
	m.setStatus("environment: "+name, false)
	if m.workspace == nil {
		return m, nil
	}
	return m, saveEnvironmentsCmd(m.workspace, m.envs)
}

// syncEditorEnvironment feeds current variable names into the editor's
// {{var}} completion.
func (m *Model) syncEditorEnvironment() {
	vars := m.activeVars()
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names)
	m.request.SetEnvironment(names, osEnvNames())
}

// expandHome resolves a leading ~ so shell-style paths work in commands.
func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

// osEnvNames returns sorted process environment variable names.
func osEnvNames() []string {
	environ := os.Environ()
	names := make([]string, 0, len(environ))
	for _, kv := range environ {
		if name, _, found := strings.Cut(kv, "="); found {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

type statusEntry struct {
	text  string
	isErr bool
}

const maxMessages = 100

func (m *Model) setStatus(msg string, isErr bool) {
	m.statusMsg = msg
	m.statusIsErr = isErr
	m.logMessage(msg, isErr)
}

func (m *Model) logMessage(msg string, isErr bool) {
	m.messages = append(m.messages, statusEntry{text: msg, isErr: isErr})
	if len(m.messages) > maxMessages {
		m.messages = m.messages[len(m.messages)-maxMessages:]
	}
}

// activeVars returns the vars of the active environment, or nil.
func (m Model) activeVars() map[string]string {
	if env := m.envs.ActiveEnv(); env != nil {
		return env.Vars
	}
	return nil
}

// startSend resolves {{var}} placeholders against the active environment and
// fires the request off, storing the cancel func so esc can abort the send.
// Resolution happens on an in-memory copy only — resolved secrets are never
// written back to the tree or to disk.
func (m Model) startSend() (tea.Model, tea.Cmd) {
	req := m.request.CurrentRequest()
	if req == nil || m.sending {
		return m, nil
	}

	resolved, missing := core.ResolveRequest(req, m.activeVars(), os.LookupEnv)
	if len(missing) > 0 {
		env := m.envs.Active
		if env == "" {
			env = "none"
		}
		err := fmt.Errorf("undefined variables: %s (active environment: %s)",
			strings.Join(missing, ", "), env)
		m.logMessage(err.Error(), true)
		m.response.SetResponse(requestLabel(req), nil, err)
		return m, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.sending = true
	m.cancelSend = cancel
	return m, tea.Batch(m.response.StartSending(),
		sendRequestCmd(ctx, resolved, requestLabel(req)))
}

func requestLabel(req *core.Request) string {
	return req.Method + " " + req.Name
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Any keypress clears the previous transient status message.
	m.statusMsg = ""

	if m.showHelp {
		switch {
		case key.Matches(msg, m.keys.Down):
			m.helpScroll++
		case key.Matches(msg, m.keys.Up):
			m.helpScroll--
		case key.Matches(msg, m.keys.Help),
			key.Matches(msg, m.keys.Escape),
			key.Matches(msg, m.keys.Quit):
			m.showHelp = false
			m.helpScroll = 0
		}
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
		if rows, visible := m.helpRows(); m.helpScroll > len(rows)-visible {
			m.helpScroll = max(len(rows)-visible, 0)
		}
		return m, nil
	}

	if m.showMessages {
		switch {
		case key.Matches(msg, m.keys.Messages),
			key.Matches(msg, m.keys.Escape),
			key.Matches(msg, m.keys.Quit):
			m.showMessages = false
		}
		return m, nil
	}

	if m.pendingImport != "" {
		return m.handleImportPromptKey(msg)
	}

	if m.cmdline.active {
		return m.handleCmdlineKey(msg)
	}

	// Insert mode and pane-local inputs (search, rename) capture everything
	// except ctrl+c, so typed text never triggers global shortcuts.
	captured := (m.focus == paneRequest && m.request.Editing()) ||
		(m.focus == paneCollections && m.collections.Capturing()) ||
		(m.focus == paneResponse && m.response.Searching())
	if captured {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m.routeToFocused(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Command):
		return m, m.cmdline.open()

	case key.Matches(msg, m.keys.Send):
		return m.startSend()

	case key.Matches(msg, m.keys.Escape):
		if m.sending && m.cancelSend != nil {
			m.cancelSend()
		}
		return m, nil

	case key.Matches(msg, m.keys.Help):
		m.showHelp = true
		return m, nil

	case key.Matches(msg, m.keys.Messages):
		m.showMessages = true
		return m, nil

	case key.Matches(msg, m.keys.ToggleSidebar):
		m.sidebarVisible = !m.sidebarVisible
		m.applyLayout()
		return m, nil

	case key.Matches(msg, m.keys.Zoom):
		m.zoomed = !m.zoomed
		m.applyLayout()
		return m, nil

	case key.Matches(msg, m.keys.GrowPane):
		m.resizeFocused(2)
		return m, nil

	case key.Matches(msg, m.keys.ShrinkPane):
		m.resizeFocused(-2)
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

// resizeFocused widens (+) or narrows (-) the focused pane by delta columns:
// the collections sidebar has its own width, request/response share a split.
func (m *Model) resizeFocused(delta int) {
	switch m.focus {
	case paneCollections:
		m.sidebarDelta += delta
	case paneRequest:
		m.splitDelta += delta
	case paneResponse:
		m.splitDelta -= delta
	}
	m.applyLayout()
}

func (m *Model) applyLayout() {
	m.sizes = layout(m.width, m.height, layoutOptions{
		sidebarVisible: m.sidebarVisible,
		sidebarDelta:   m.sidebarDelta,
		splitDelta:     m.splitDelta,
	})
	if m.zoomed {
		// The focused pane takes the whole content area.
		full := PanelSize{Width: m.width, Height: m.height - statusBarHeight}
		m.sizes.Collections, m.sizes.Request, m.sizes.Response = full, full, full
	}
	m.collections.SetSize(m.sizes.Collections.Width, m.sizes.Collections.Height)
	m.request.SetSize(m.sizes.Request.Width, m.sizes.Request.Height)
	m.response.SetSize(m.sizes.Response.Width, m.sizes.Response.Height)

	if m.sizes.Mode == ModeDouble && !m.sidebarVisible && m.focus == paneCollections {
		m.focus = m.lastMain
	}
	m.applyFocus()
}

func (m Model) View() tea.View {
	var content string
	if m.width > 0 && m.height > 0 {
		switch {
		case m.showHelp:
			content = m.helpView()
		case m.showMessages:
			content = m.messagesView()
		default:
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
	if m.zoomed {
		switch m.focus {
		case paneRequest:
			return m.request.View()
		case paneResponse:
			return m.response.View()
		default:
			return m.collections.View()
		}
	}

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

// helpAvail returns the inner space of the help overlay box: the terminal
// minus status bar, box border, padding, title, and footer rows.
func (m Model) helpAvail() (width, height int) {
	return max(m.width-10, 20), max(m.height-statusBarHeight-8, 3)
}

// helpRows lays the enabled bindings out in as many aligned columns as the
// terminal width allows, filling column-major. It returns the row lines and
// how many of them fit on screen at once.
func (m Model) helpRows() ([]string, int) {
	var rendered []string
	entryWidth := 0
	for _, group := range m.keys.FullHelp() {
		for _, binding := range group {
			if !binding.Enabled() {
				continue
			}
			entry := m.theme.HelpKey.Render(padRight(binding.Help().Key, 10)) +
				m.theme.HelpDesc.Render(binding.Help().Desc)
			rendered = append(rendered, entry)
			entryWidth = max(entryWidth, lipgloss.Width(entry))
		}
	}

	availWidth, availHeight := m.helpAvail()
	const gap = 3
	maxCols := max((availWidth+gap)/(entryWidth+gap), 1)

	// Start compact and add columns until the list fits the height (or the
	// width runs out); leftover rows scroll.
	cols := min(maxCols, 2)
	rows := (len(rendered) + cols - 1) / cols
	for rows > availHeight && cols < maxCols {
		cols++
		rows = (len(rendered) + cols - 1) / cols
	}

	// Fill column-major, padding every cell to entryWidth so columns align.
	lines := make([]string, rows)
	for col := 0; col < cols; col++ {
		for row := 0; row < rows; row++ {
			idx := col*rows + row
			if idx >= len(rendered) {
				continue
			}
			cell := rendered[idx]
			if col < cols-1 {
				cell += strings.Repeat(" ", entryWidth-lipgloss.Width(cell)+gap)
			}
			lines[row] += cell
		}
	}
	return lines, availHeight
}

func (m Model) helpView() string {
	rows, visible := m.helpRows()
	area := m.height - statusBarHeight

	scroll := min(max(m.helpScroll, 0), max(len(rows)-visible, 0))
	shown := rows[scroll:min(scroll+visible, len(rows))]

	footer := ""
	if len(rows) > visible {
		footer = "\n\n" + m.theme.Muted.Render(fmt.Sprintf(
			"j/k scroll · %d–%d/%d", scroll+1, scroll+len(shown), len(rows)))
	}

	box := m.theme.HelpBox.Render(
		m.theme.HelpTitle.Render("payk — keybindings") + "\n" +
			strings.Join(shown, "\n") + footer)
	box = clampBlock(box, m.width, area)

	return lipgloss.Place(m.width, area, lipgloss.Center, lipgloss.Center, box)
}

// messagesView renders the full-text message log, newest last, wrapped to
// the terminal width — the place to read what the status bar truncated.
func (m Model) messagesView() string {
	area := m.height - statusBarHeight
	innerWidth := max(m.width-8, 20)

	var lines []string
	if len(m.messages) == 0 {
		lines = append(lines, m.theme.Muted.Render("no messages yet"))
	}
	for _, entry := range m.messages {
		style := m.theme.HelpDesc
		marker := m.theme.Muted.Render("· ")
		if entry.isErr {
			style = m.theme.HelpDesc.Foreground(m.theme.Flavor.Red())
			marker = m.theme.Status(500).Render("✗ ")
		}
		wrapped := ansi.Hardwrap(style.Render(entry.text), innerWidth-2, true)
		for i, line := range strings.Split(wrapped, "\n") {
			if i == 0 {
				lines = append(lines, marker+line)
			} else {
				lines = append(lines, "  "+line)
			}
		}
	}
	// Keep the newest messages when the log outgrows the overlay.
	maxLines := max(area-6, 3)
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	box := m.theme.HelpBox.Render(
		m.theme.HelpTitle.Render("messages") + "\n" + strings.Join(lines, "\n") +
			"\n\n" + m.theme.Muted.Render("m / esc to close"))
	box = clampBlock(box, m.width, area)
	return lipgloss.Place(m.width, area, lipgloss.Center, lipgloss.Center, box)
}

// clampBlock hard-limits a rendered block to width x height cells so an
// overlay can never overflow the terminal.
func clampBlock(block string, width, height int) string {
	lines := strings.Split(block, "\n")
	if len(lines) > height {
		lines = lines[:max(height, 0)]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "…")
	}
	return strings.Join(lines, "\n")
}

func (m Model) statusView() string {
	if m.pendingImport != "" {
		prompt := m.theme.StatusBadge.Render("import") + " " +
			m.theme.StatusHint.Render("paste looks like "+pasteKindLabel(m.pendingImportKind)+" — import as request? (y/n)")
		return m.theme.StatusBar.Width(m.width).MaxWidth(m.width).Render(prompt)
	}
	if m.cmdline.active {
		return m.theme.StatusBar.Width(m.width).MaxWidth(m.width).Render(" " + m.cmdline.view())
	}

	badge := m.theme.StatusBadge.Render("payk")
	focus := m.theme.StatusFocus.Render(m.focus.String())

	segments := []string{badge, focus}
	if m.envs.Active != "" {
		segments = append(segments, m.theme.StatusFocus.Render("env:"+m.envs.Active))
	}
	if m.zoomed {
		segments = append(segments, m.theme.StatusFocus.Render("zoom"))
	}

	// The trailing segment (message or hint) shrinks to whatever width the
	// fixed segments leave over, so the bar never overflows.
	used := lipgloss.Width(lipgloss.JoinHorizontal(lipgloss.Top, segments...))
	remaining := max(m.width-used-1, 0)
	if m.statusMsg != "" {
		style := m.theme.StatusHint
		if m.statusIsErr {
			style = style.Foreground(m.theme.Flavor.Red())
		}
		segments = append(segments, style.Render(ansi.Truncate(m.statusMsg, remaining, "…")))
	} else {
		segments = append(segments, m.theme.StatusHint.Render(
			ansi.Truncate("? help · : cmd · z zoom · q quit", remaining, "…")))
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Top, segments...)
	return m.theme.StatusBar.Width(m.width).MaxWidth(m.width).Render(bar)
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s + " "
	}
	return s + strings.Repeat(" ", width-len(s))
}
