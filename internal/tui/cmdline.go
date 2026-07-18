package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// cmdLine is the vim-style ":" command input shown in place of the status
// bar while active.
type cmdLine struct {
	input  textinput.Model
	active bool
}

func newCmdLine() cmdLine {
	ti := textinput.New()
	ti.Prompt = ":"
	return cmdLine{input: ti}
}

func (c *cmdLine) open() tea.Cmd {
	c.active = true
	c.input.SetValue("")
	return c.input.Focus()
}

func (c *cmdLine) close() {
	c.active = false
	c.input.Blur()
}

func (c *cmdLine) value() string {
	return c.input.Value()
}

func (c *cmdLine) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return cmd
}

func (c *cmdLine) view() string {
	return c.input.View()
}
