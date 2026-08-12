package theme

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	catppuccin "github.com/catppuccin/go"
)

func contrast(fg, bg color.Color) float64 {
	high, low := relativeLuminance(fg), relativeLuminance(bg)
	if high < low {
		high, low = low, high
	}
	return (high + 0.05) / (low + 0.05)
}

func flavors() []catppuccin.Flavor {
	return []catppuccin.Flavor{
		catppuccin.Latte, catppuccin.Frappe, catppuccin.Macchiato, catppuccin.Mocha,
	}
}

// Every flavor has to clear these floors. Latte used to fall through them:
// muted text sat at 2.83:1, counts at 2.30:1 and chips at 1.75:1, because the
// neutral ramp is tuned for dark bases.
func TestSecondaryTextStaysReadable(t *testing.T) {
	cases := []struct {
		name  string
		floor float64
		style func(*Theme) lipgloss.Style
	}{
		{"muted", 3.5, func(t *Theme) lipgloss.Style { return t.Muted }},
		{"inactive tab", 3.5, func(t *Theme) lipgloss.Style { return t.TabInactive }},
		{"pane title", 3.5, func(t *Theme) lipgloss.Style { return t.PaneTitle }},
		{"tree count", 2.8, func(t *Theme) lipgloss.Style { return t.TreeCount }},
		{"status hint", 3.5, func(t *Theme) lipgloss.Style { return t.StatusHint }},
		{"chip", 3.0, func(t *Theme) lipgloss.Style { return t.Chip }},
		{"pill", 3.2, func(t *Theme) lipgloss.Style { return t.PillMuted }},
		{"badge", 3.2, func(t *Theme) lipgloss.Style { return t.Badge }},
		{"bar", 3.2, func(t *Theme) lipgloss.Style { return t.Bar }},
	}

	for _, flavor := range flavors() {
		theme := New(flavor)
		for _, tc := range cases {
			style := tc.style(theme)
			// An unset background is NoColor, which reads as opaque black;
			// those styles are drawn straight onto the pane background.
			bg := style.GetBackground()
			if _, unset := bg.(lipgloss.NoColor); unset {
				bg = theme.Base
			}
			got := contrast(style.GetForeground(), bg)
			if got < tc.floor {
				t.Errorf("%s: %s reads at %.2f:1, want at least %.2f:1",
					flavor.Name(), tc.name, got, tc.floor)
			}
		}
	}
}

// Titles, muted text and counts have to stay three separate steps apart, or a
// pane reads as one flat block. Moving light flavors down the ramp can collapse
// two roles onto the same color if each is not moved.
func TestSecondaryTextKeepsItsHierarchy(t *testing.T) {
	for _, flavor := range flavors() {
		theme := New(flavor)
		title := contrast(theme.PaneTitle.GetForeground(), theme.Base)
		muted := contrast(theme.Muted.GetForeground(), theme.Base)
		count := contrast(theme.TreeCount.GetForeground(), theme.Base)
		if title <= muted || muted <= count {
			t.Errorf("%s: title %.2f:1, muted %.2f:1, count %.2f:1 — each step must be weaker than the one above",
				flavor.Name(), title, muted, count)
		}
	}
}

func TestLightFlavorTakesDarkerRampStep(t *testing.T) {
	latte := New(catppuccin.Latte)
	if !latte.light {
		t.Fatal("Latte should be detected as a light flavor")
	}
	if got := latte.Muted.GetForeground(); sameColor(got, catppuccin.Latte.Overlay1()) {
		t.Error("Latte muted text still uses Overlay1, the low-contrast step")
	}

	mocha := New(catppuccin.Mocha)
	if mocha.light {
		t.Fatal("Mocha should not be detected as a light flavor")
	}
	if got := mocha.Muted.GetForeground(); !sameColor(got, catppuccin.Mocha.Overlay1()) {
		t.Error("dark flavors must keep the original ramp step")
	}
}

func sameColor(a, b color.Color) bool {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	return ar == br && ag == bg && ab == bb
}
