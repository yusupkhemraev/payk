// Package panels contains the three UI panels: collections tree, request
// editor, and response viewer. Each panel owns its state and rendering; the
// root model only routes messages and sets sizes.
package panels

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yusupkhemraev/payk/internal/config"
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

// frame renders a pane of exactly width x height in the configured chrome
// style. Overflow is clipped so resizing never breaks rendering.
func frame(t *theme.Theme, title, meta string, focused bool, width, height int, body string) string {
	if width < 6 || height < 3 {
		return ""
	}
	switch t.Chrome {
	case config.ChromePlain:
		return framePlain(t, title, meta, focused, width, height, body)
	case config.ChromeClassic:
		return frameClassic(t, title, meta, focused, width, height, body)
	}
	return frameBoxed(t, title, meta, focused, width, height, body)
}

// frameBoxed puts the title in the top border on the right, with any meta
// (pills) to its left, so the header costs no content row.
func frameBoxed(t *theme.Theme, title, meta string, focused bool, width, height int, body string) string {

	border := t.Border
	if focused {
		border = t.BorderOn
	}
	inner := width - 2

	// Build the top border right to left: corner, title, meta, then dashes.
	right := " " + t.PaneTitle.Render(title) + " "
	if focused {
		right = " " + t.PaneActive.Render(title) + " "
	}
	if meta != "" {
		right = " " + meta + right
	}
	fill := max(inner-ansi.StringWidth(right), 1)
	top := border.Render("╭"+strings.Repeat("─", fill)) + right + border.Render("╮")

	lines := []string{top}
	side := border.Render("│")
	for _, line := range strings.Split(body, "\n") {
		if len(lines) >= height-1 {
			break
		}
		lines = append(lines, side+Pad(" "+ansi.Truncate(line, inner-2, "…"), inner)+side)
	}
	for len(lines) < height-1 {
		lines = append(lines, side+strings.Repeat(" ", inner)+side)
	}
	lines = append(lines, border.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return strings.Join(lines, "\n")
}

// frameClassic keeps a border but gives the title its own row above a rule,
// the way payk looked before the redesign.
func frameClassic(t *theme.Theme, title, meta string, focused bool, width, height int, body string) string {
	border := t.Border
	if focused {
		border = t.BorderOn
	}
	inner := width - 2

	head := t.PaneTitle.Render(title)
	if focused {
		head = t.PaneActive.Render(title)
	}
	if meta != "" {
		head = SplitRow(head, meta, inner-2)
	}

	content := append([]string{head, t.Separator.Render(strings.Repeat("─", inner-2))},
		strings.Split(body, "\n")...)

	lines := []string{border.Render("╭" + strings.Repeat("─", inner) + "╮")}
	side := border.Render("│")
	for _, line := range content {
		if len(lines) >= height-1 {
			break
		}
		lines = append(lines, side+Pad(" "+ansi.Truncate(line, inner-2, "…"), inner)+side)
	}
	for len(lines) < height-1 {
		lines = append(lines, side+strings.Repeat(" ", inner)+side)
	}
	lines = append(lines, border.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return strings.Join(lines, "\n")
}

// framePlain drops borders for an accent focus bar, freeing two columns.
func framePlain(t *theme.Theme, title, meta string, focused bool, width, height int, body string) string {
	bar := " "
	head := t.PaneTitle.Render(title)
	if focused {
		bar = t.FocusBar.Render("▎")
		head = t.PaneActive.Render(title)
	}
	inner := width - 2
	if meta != "" {
		head = SplitRow(head, meta, inner)
	}

	content := append([]string{ansi.Truncate(head, inner, "…"),
		t.Separator.Render(strings.Repeat("─", inner))},
		strings.Split(body, "\n")...)

	out := make([]string, height)
	for i := range height {
		line := ""
		if i < len(content) {
			line = ansi.Truncate(content[i], inner, "…")
		}
		out[i] = bar + " " + Pad(line, inner)
	}
	return strings.Join(out, "\n")
}
