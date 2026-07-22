package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// cmdSuggestion is one completion candidate for the command line.
type cmdSuggestion struct {
	// full replaces the whole input when accepted.
	full  string
	label string
	desc  string
}

// cmdLine is the vim-style ":" command input shown in place of the status
// bar while active, with context-aware completion.
type cmdLine struct {
	input  textinput.Model
	active bool

	suggestions []cmdSuggestion
	suggIdx     int
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
	c.suggestions = nil
	c.suggIdx = 0
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

func (c *cmdLine) setSuggestions(suggestions []cmdSuggestion) {
	c.suggestions = suggestions
	if c.suggIdx >= len(suggestions) {
		c.suggIdx = 0
	}
}

func (c *cmdLine) cycle(delta int) {
	if len(c.suggestions) == 0 {
		return
	}
	c.suggIdx = (c.suggIdx + delta + len(c.suggestions)) % len(c.suggestions)
}

// accept replaces the input with the selected suggestion.
func (c *cmdLine) accept() bool {
	if len(c.suggestions) == 0 {
		return false
	}
	c.input.SetValue(c.suggestions[c.suggIdx].full)
	c.input.CursorEnd()
	return true
}

func (c *cmdLine) view() string {
	return c.input.View()
}
