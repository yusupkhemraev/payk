// Package keymap is the central registry of all key bindings, so they stay
// discoverable in the help overlay and rebindable from config later. Panels
// receive the KeyMap instead of defining their own bindings.
package keymap

import "charm.land/bubbles/v2/key"

// KeyMap holds every binding used across the TUI.
type KeyMap struct {
	Quit    key.Binding
	Help    key.Binding
	Escape  key.Binding
	Command key.Binding

	FocusLeft     key.Binding
	FocusRight    key.Binding
	NextPane      key.Binding
	PrevPane      key.Binding
	ToggleSidebar key.Binding

	Up       key.Binding
	Down     key.Binding
	Top      key.Binding
	Bottom   key.Binding
	HalfDown key.Binding
	HalfUp   key.Binding
	Select   key.Binding
	Search   key.Binding

	TabNext key.Binding
	TabPrev key.Binding

	Insert    key.Binding
	AddRow    key.Binding
	DeleteRow key.Binding

	Send key.Binding
	Edit key.Binding
}

// Default returns the standard vim-style key map.
func Default() KeyMap {
	return KeyMap{
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
		HalfDown: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "half page down"),
		),
		HalfUp: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "half page up"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", "o"),
			key.WithHelp("enter", "open / toggle / cycle"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search in pane"),
			key.WithDisabled(), // wired in a later milestone
		),

		TabNext: key.NewBinding(
			key.WithKeys("]"),
			key.WithHelp("]", "next editor tab"),
		),
		TabPrev: key.NewBinding(
			key.WithKeys("["),
			key.WithHelp("[", "previous editor tab"),
		),

		Insert: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "edit field (insert mode)"),
		),
		AddRow: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add row"),
		),
		DeleteRow: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete row"),
		),

		Send: key.NewBinding(
			key.WithKeys("space"),
			key.WithHelp("space", "send request"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "open in $EDITOR"),
			key.WithDisabled(), // wired in a later milestone
		),
	}
}

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.NextPane, k.Quit}
}

// FullHelp implements help.KeyMap; columns group related bindings for the
// help overlay.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.FocusLeft, k.FocusRight, k.NextPane, k.PrevPane, k.ToggleSidebar},
		{k.Up, k.Down, k.Top, k.Bottom, k.HalfDown, k.HalfUp, k.Select, k.Search},
		{k.TabNext, k.TabPrev, k.Insert, k.AddRow, k.DeleteRow},
		{k.Send, k.Edit, k.Command, k.Help, k.Quit},
	}
}
