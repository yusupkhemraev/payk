package tui

import "charm.land/bubbles/v2/key"

// keyMap is the central registry of all bindings, so they stay discoverable
// in the help overlay and rebindable from config later.
type keyMap struct {
	Quit    key.Binding
	Help    key.Binding
	Escape  key.Binding
	Command key.Binding

	FocusLeft     key.Binding
	FocusRight    key.Binding
	NextPane      key.Binding
	PrevPane      key.Binding
	ToggleSidebar key.Binding

	Up     key.Binding
	Down   key.Binding
	Top    key.Binding
	Bottom key.Binding
	Search key.Binding

	Send key.Binding
	Edit key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back to normal mode"),
		),
		Command: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":", "command line"),
			key.WithDisabled(), // wired in a later milestone
		),

		FocusLeft: key.NewBinding(
			key.WithKeys("h", "ctrl+h"),
			key.WithHelp("h", "focus pane left"),
		),
		FocusRight: key.NewBinding(
			key.WithKeys("l", "ctrl+l"),
			key.WithHelp("l", "focus pane right"),
		),
		NextPane: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next pane"),
		),
		PrevPane: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "previous pane"),
		),
		ToggleSidebar: key.NewBinding(
			key.WithKeys("ctrl+b"),
			key.WithHelp("ctrl+b", "toggle collections"),
		),

		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j", "down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("gg", "go to top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "go to bottom"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search in pane"),
			key.WithDisabled(), // wired in a later milestone
		),

		Send: key.NewBinding(
			key.WithKeys("space"),
			key.WithHelp("space", "send request"),
			key.WithDisabled(), // wired in a later milestone
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "open in $EDITOR"),
			key.WithDisabled(), // wired in a later milestone
		),
	}
}

// ShortHelp implements help.KeyMap.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.NextPane, k.Quit}
}

// FullHelp implements help.KeyMap; columns group related bindings for the
// help overlay.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.FocusLeft, k.FocusRight, k.NextPane, k.PrevPane, k.ToggleSidebar},
		{k.Up, k.Down, k.Top, k.Bottom, k.Search},
		{k.Send, k.Edit, k.Command, k.Help, k.Quit},
	}
}
