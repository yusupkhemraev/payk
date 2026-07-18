package tui

import "testing"

func TestLayoutBreakpoints(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		height  int
		sidebar bool
		want    LayoutMode
	}{
		{"wide is triple", 120, 40, true, ModeTriple},
		{"very wide is triple", 200, 50, true, ModeTriple},
		{"medium is double", 119, 30, true, ModeDouble},
		{"medium lower bound is double", 80, 24, true, ModeDouble},
		{"narrow is single", 79, 24, true, ModeSingle},
		{"tiny is single", 60, 20, true, ModeSingle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := layout(tt.width, tt.height, tt.sidebar)
			if got.Mode != tt.want {
				t.Errorf("layout(%d, %d) mode = %v, want %v", tt.width, tt.height, got.Mode, tt.want)
			}
		})
	}
}

func TestLayoutTripleWidthsSumToTerminalWidth(t *testing.T) {
	for _, width := range []int{120, 137, 200} {
		sizes := layout(width, 40, true)
		sum := sizes.Collections.Width + sizes.Request.Width + sizes.Response.Width
		if sum != width {
			t.Errorf("width %d: pane widths sum to %d", width, sum)
		}
	}
}

func TestLayoutDoubleWidths(t *testing.T) {
	sizes := layout(100, 30, true)
	if sizes.Collections.Width+sizes.Request.Width != 100 {
		t.Errorf("sidebar visible: widths %d + %d don't sum to 100",
			sizes.Collections.Width, sizes.Request.Width)
	}

	sizes = layout(100, 30, false)
	if sizes.Collections.Width != 0 {
		t.Errorf("sidebar hidden: collections width = %d, want 0", sizes.Collections.Width)
	}
	if sizes.Request.Width+sizes.Response.Width != 100 {
		t.Errorf("sidebar hidden: widths %d + %d don't sum to 100",
			sizes.Request.Width, sizes.Response.Width)
	}
}

func TestLayoutReservesStatusBarRow(t *testing.T) {
	sizes := layout(120, 40, true)
	if sizes.Collections.Height != 39 {
		t.Errorf("pane height = %d, want 39", sizes.Collections.Height)
	}
}

func TestLayoutDegenerateSizesDoNotPanic(t *testing.T) {
	for _, tc := range [][2]int{{0, 0}, {-1, 10}, {10, -1}, {1, 1}, {5, 2}} {
		sizes := layout(tc[0], tc[1], true)
		for _, s := range []PanelSize{sizes.Collections, sizes.Request, sizes.Response} {
			if s.Width < 0 || s.Height < 0 {
				t.Errorf("layout(%d, %d) produced negative size %+v", tc[0], tc[1], s)
			}
		}
	}
}
