// Package panels contains the three UI panels: collections tree, request
// editor, and response viewer. Each panel owns its state and rendering; the
// root model only routes messages and sets sizes.
package panels

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yusupkhemraev/payk/internal/tui/theme"
)

// StatusNote asks the root model to flash a transient status message.
type StatusNote struct {
	Text  string
	IsErr bool
}

func noteCmd(text string, isErr bool) tea.Cmd {
	return func() tea.Msg { return StatusNote{Text: text, IsErr: isErr} }
}

// Pad extends a line to exactly w cells, truncating what does not fit.
func Pad(s string, w int) string {
	if d := w - ansi.StringWidth(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return ansi.Truncate(s, w, "…")
}

// SplitRow puts left and right content on one line of width w, right-aligned.
func SplitRow(left, right string, w int) string {
	gap := w - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		return Pad(left, w)
	}
	return left + strings.Repeat(" ", gap) + right
}

// frame renders a frameless pane of exactly width x height: an accent focus
// bar on the left, a title row with an optional right-aligned meta, a rule,
// and the body. Overflow is clipped so resizing never breaks rendering.
func frame(t *theme.Theme, title, meta string, focused bool, width, height int, body string) string {
	if width < 4 || height < 3 {
		return ""
	}

	bar := " "
	titleStyle := t.PaneTitle
	if focused {
		bar = t.FocusBar.Render("▎")
		titleStyle = t.PaneActive
	}

	inner := width - 2
	head := titleStyle.Render(title)
	if meta != "" {
		head = SplitRow(head, t.TreeCount.Render(meta), inner)
	}

	content := []string{
		ansi.Truncate(head, inner, "…"),
		t.Separator.Render(strings.Repeat("─", inner)),
	}
	for _, line := range strings.Split(body, "\n") {
		if len(content) >= height {
			break
		}
		content = append(content, ansi.Truncate(line, inner, "…"))
	}

	out := make([]string, height)
	for i := range height {
		line := ""
		if i < len(content) {
			line = content[i]
		}
		out[i] = bar + " " + Pad(line, inner)
	}
	return strings.Join(out, "\n")
}
