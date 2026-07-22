// Package theme centralizes every lipgloss style used by the TUI.
// Views must not create inline styles; they pull them from a Theme so the
// whole app can switch Catppuccin flavors (Latte/Frappe/Macchiato/Mocha)
// from config without touching view code.
package theme

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	catppuccin "github.com/catppuccin/go"
)

type Theme struct {
	Flavor catppuccin.Flavor

	Base    color.Color
	Mantle  color.Color
	Surface color.Color
	Overlay color.Color
	Text    color.Color
	Subtext color.Color
	Accent  color.Color

	panelBorder        lipgloss.Style
	panelBorderFocused lipgloss.Style
	panelTitle         lipgloss.Style
	panelTitleFocused  lipgloss.Style

	StatusBar   lipgloss.Style
	StatusBadge lipgloss.Style
	StatusFocus lipgloss.Style
	StatusHint  lipgloss.Style

	HelpBox   lipgloss.Style
	HelpTitle lipgloss.Style
	HelpKey   lipgloss.Style
	HelpDesc  lipgloss.Style

	Muted    lipgloss.Style
	Selected lipgloss.Style

	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	FieldLabel  lipgloss.Style

	TreeCollection lipgloss.Style
	TreeFolder     lipgloss.Style
	TreeCount      lipgloss.Style
	Separator      lipgloss.Style

	statusOK       lipgloss.Style
	statusRedirect lipgloss.Style
	statusClient   lipgloss.Style
	statusServer   lipgloss.Style

	methods map[string]lipgloss.Style
}

func New(flavor catppuccin.Flavor) *Theme {
	t := &Theme{
		Flavor:  flavor,
		Base:    flavor.Base(),
		Mantle:  flavor.Mantle(),
		Surface: flavor.Surface0(),
		Overlay: flavor.Overlay0(),
		Text:    flavor.Text(),
		Subtext: flavor.Subtext0(),
		Accent:  flavor.Mauve(),
	}

	border := lipgloss.RoundedBorder()

	t.panelBorder = lipgloss.NewStyle().
		Border(border).
		BorderForeground(flavor.Surface1())
	t.panelBorderFocused = t.panelBorder.
		BorderForeground(flavor.Mauve())

	t.panelTitle = lipgloss.NewStyle().
		Foreground(flavor.Subtext0()).
		Padding(0, 1)
	t.panelTitleFocused = t.panelTitle.
		Foreground(flavor.Mauve()).
		Bold(true)

	t.StatusBar = lipgloss.NewStyle().
		Background(flavor.Mantle()).
		Foreground(flavor.Subtext0())
	t.StatusBadge = lipgloss.NewStyle().
		Background(flavor.Mauve()).
		Foreground(flavor.Crust()).
		Bold(true).
		Padding(0, 1)
	t.StatusFocus = lipgloss.NewStyle().
		Background(flavor.Surface0()).
		Foreground(flavor.Text()).
		Padding(0, 1)
	t.StatusHint = lipgloss.NewStyle().
		Background(flavor.Mantle()).
		Foreground(flavor.Overlay1()).
		Padding(0, 1)

	t.HelpBox = lipgloss.NewStyle().
		Border(border).
		BorderForeground(flavor.Mauve()).
		Padding(1, 3)
	t.HelpTitle = lipgloss.NewStyle().
		Foreground(flavor.Mauve()).
		Bold(true).
		MarginBottom(1)
	t.HelpKey = lipgloss.NewStyle().
		Foreground(flavor.Peach()).
		Bold(true)
	t.HelpDesc = lipgloss.NewStyle().
		Foreground(flavor.Subtext1())

	t.Muted = lipgloss.NewStyle().Foreground(flavor.Overlay1())
	t.Selected = lipgloss.NewStyle().
		Background(flavor.Surface1()).
		Foreground(flavor.Text()).
		Bold(true)

	t.TabActive = lipgloss.NewStyle().
		Foreground(flavor.Mauve()).
		Bold(true).
		Underline(true)
	t.TabInactive = lipgloss.NewStyle().Foreground(flavor.Overlay1())
	t.FieldLabel = lipgloss.NewStyle().Foreground(flavor.Subtext0())

	t.TreeCollection = lipgloss.NewStyle().
		Foreground(flavor.Sapphire()).
		Bold(true)
	t.TreeFolder = lipgloss.NewStyle().Foreground(flavor.Text())
	t.TreeCount = lipgloss.NewStyle().Foreground(flavor.Overlay0())
	t.Separator = lipgloss.NewStyle().Foreground(flavor.Surface1())

	t.statusOK = lipgloss.NewStyle().Foreground(flavor.Green()).Bold(true)
	t.statusRedirect = lipgloss.NewStyle().Foreground(flavor.Yellow()).Bold(true)
	t.statusClient = lipgloss.NewStyle().Foreground(flavor.Peach()).Bold(true)
	t.statusServer = lipgloss.NewStyle().Foreground(flavor.Red()).Bold(true)

	t.methods = map[string]lipgloss.Style{
		"GET":     lipgloss.NewStyle().Foreground(flavor.Green()).Bold(true),
		"POST":    lipgloss.NewStyle().Foreground(flavor.Blue()).Bold(true),
		"PUT":     lipgloss.NewStyle().Foreground(flavor.Yellow()).Bold(true),
		"PATCH":   lipgloss.NewStyle().Foreground(flavor.Peach()).Bold(true),
		"DELETE":  lipgloss.NewStyle().Foreground(flavor.Red()).Bold(true),
		"HEAD":    lipgloss.NewStyle().Foreground(flavor.Teal()).Bold(true),
		"OPTIONS": lipgloss.NewStyle().Foreground(flavor.Mauve()).Bold(true),
	}

	return t
}

// Default returns the Catppuccin Mocha theme.
func Default() *Theme {
	return New(catppuccin.Mocha)
}

// ByName returns the theme for a Catppuccin flavor name (case-insensitive),
// falling back to Mocha for unknown names.
func ByName(name string) *Theme {
	if flavor := catppuccin.Variant(name); flavor != nil {
		return New(flavor)
	}
	return Default()
}

func (t *Theme) PanelBorder(focused bool) lipgloss.Style {
	if focused {
		return t.panelBorderFocused
	}
	return t.panelBorder
}

func (t *Theme) PanelTitle(focused bool) lipgloss.Style {
	if focused {
		return t.panelTitleFocused
	}
	return t.panelTitle
}

func (t *Theme) Method(method string) lipgloss.Style {
	if s, ok := t.methods[strings.ToUpper(method)]; ok {
		return s
	}
	return lipgloss.NewStyle().Foreground(t.Subtext).Bold(true)
}

func (t *Theme) Status(code int) lipgloss.Style {
	switch {
	case code >= 500:
		return t.statusServer
	case code >= 400:
		return t.statusClient
	case code >= 300:
		return t.statusRedirect
	default:
		return t.statusOK
	}
}

// ChromaStyle returns the chroma syntax highlighting style name matching the
// current Catppuccin flavor.
func (t *Theme) ChromaStyle() string {
	return "catppuccin-" + t.Flavor.Name()
}
