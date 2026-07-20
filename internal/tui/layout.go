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

// minPaneWidth keeps a resized pane usable.
const minPaneWidth = 20

// layoutOptions tune the computed layout: sidebar visibility (ModeDouble
// only) and user-driven width adjustments in columns.
type layoutOptions struct {
	sidebarVisible bool
	// sidebarDelta widens (+) or narrows (-) the collections pane.
	sidebarDelta int
	// splitDelta moves the request/response boundary: + widens request.
	splitDelta int
}

// layout is the single place panel geometry is computed from the terminal
// size.
func layout(width, height int, opts layoutOptions) PanelSizes {
	if width <= 0 || height <= statusBarHeight {
		return PanelSizes{Mode: ModeSingle}
	}

	paneHeight := height - statusBarHeight

	switch {
	case width >= tripleBreakpoint:
		sidebar := clamp(width/4+opts.sidebarDelta, sidebarMinWidth, width/2)
		request := clamp((width-sidebar)/2+opts.splitDelta,
			minPaneWidth, width-sidebar-minPaneWidth)
		response := width - sidebar - request
		return PanelSizes{
			Mode:        ModeTriple,
			Collections: PanelSize{sidebar, paneHeight},
			Request:     PanelSize{request, paneHeight},
			Response:    PanelSize{response, paneHeight},
		}

	case width >= doubleBreakpoint:
		if opts.sidebarVisible {
			sidebar := clamp(width/4+opts.sidebarDelta, sidebarMinWidth, width/2)
			main := width - sidebar
			return PanelSizes{
				Mode:        ModeDouble,
				Collections: PanelSize{sidebar, paneHeight},
				Request:     PanelSize{main, paneHeight},
				Response:    PanelSize{main, paneHeight},
			}
		}
		request := clamp(width/2+opts.splitDelta, minPaneWidth, width-minPaneWidth)
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
