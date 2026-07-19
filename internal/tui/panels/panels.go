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

// frame renders a bordered panel box of exactly width x height with a title
// row and body content, clipping overflow so resizing never breaks rendering.
func frame(t *theme.Theme, title string, focused bool, width, height int, body string) string {
	if width < 4 || height < 3 {
		return ""
	}

	innerWidth := width - 2
	innerHeight := height - 2

	titleLine := ansi.Truncate(t.PanelTitle(focused).Render(title), innerWidth, "…")
	separator := t.Separator.Render(strings.Repeat("─", innerWidth))

	bodyLines := strings.Split(body, "\n")
	if len(bodyLines) > innerHeight-2 {
		bodyLines = bodyLines[:max(innerHeight-2, 0)]
	}
	for i, line := range bodyLines {
		bodyLines[i] = ansi.Truncate(line, innerWidth, "…")
	}

	content := titleLine + "\n" + separator
	if len(bodyLines) > 0 {
		content += "\n" + strings.Join(bodyLines, "\n")
	}

	// lipgloss v2 Width/Height are the final block size, borders included.
	return t.PanelBorder(focused).
		Width(width).
		Height(height).
		Render(content)
}
