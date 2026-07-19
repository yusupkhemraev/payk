package tui

// LayoutMode describes how many panes are visible at the current width.
type LayoutMode int

const (
	// ModeSingle shows one pane at a time (< 80 cols), switched via tab.
	ModeSingle LayoutMode = iota
	// ModeDouble shows two panes (80–119 cols); the collections tree is
	// collapsible and swaps in for the focused editor/viewer pane.
	ModeDouble
	// ModeTriple shows all three panes side by side (>= 120 cols).
	ModeTriple
)

const (
	tripleBreakpoint = 120
	doubleBreakpoint = 80

	sidebarMinWidth = 24
	sidebarMaxWidth = 42

	statusBarHeight = 1
)

// PanelSize is the outer box size of a panel, borders included.
type PanelSize struct {
	Width  int
	Height int
}

// PanelSizes is the computed layout for one terminal size. A zero-size panel
// is not visible in the current mode.
type PanelSizes struct {
	Mode        LayoutMode
	Collections PanelSize
	Request     PanelSize
	Response    PanelSize
}

// layout is the single place panel geometry is computed from the terminal
// size. sidebarVisible only matters in ModeDouble, where the collections tree
// can be toggled.
func layout(width, height int, sidebarVisible bool) PanelSizes {
	if width <= 0 || height <= statusBarHeight {
		return PanelSizes{Mode: ModeSingle}
	}

	paneHeight := height - statusBarHeight

	switch {
	case width >= tripleBreakpoint:
		sidebar := clamp(width/4, sidebarMinWidth, sidebarMaxWidth)
		request := (width - sidebar) / 2
		response := width - sidebar - request
		return PanelSizes{
			Mode:        ModeTriple,
			Collections: PanelSize{sidebar, paneHeight},
			Request:     PanelSize{request, paneHeight},
			Response:    PanelSize{response, paneHeight},
		}

	case width >= doubleBreakpoint:
		if sidebarVisible {
			sidebar := clamp(width/4, sidebarMinWidth, sidebarMaxWidth)
			main := width - sidebar
			return PanelSizes{
				Mode:        ModeDouble,
				Collections: PanelSize{sidebar, paneHeight},
				Request:     PanelSize{main, paneHeight},
				Response:    PanelSize{main, paneHeight},
			}
		}
		request := width / 2
		response := width - request
		return PanelSizes{
			Mode:     ModeDouble,
			Request:  PanelSize{request, paneHeight},
			Response: PanelSize{response, paneHeight},
		}

	default:
		full := PanelSize{width, paneHeight}
		return PanelSizes{
			Mode:        ModeSingle,
			Collections: full,
			Request:     full,
			Response:    full,
		}
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
