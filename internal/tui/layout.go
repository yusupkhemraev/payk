package tui

type LayoutMode int

const (
	// ModeSingle shows one pane at a time (< 80 cols), switched via tab.
	ModeSingle LayoutMode = iota
	// ModeDouble shows two panes (80–119 cols); the collections tree is
	// collapsible and swaps in for the focused editor/viewer pane.
	ModeDouble
	// ModeTriple shows all three panes side by side (>= 120 cols).
	ModeTriple
	// ModeStacked keeps the tree on the left and stacks the request above
	// the response, giving both bodies the full pane width.
	ModeStacked
)

const (
	tripleBreakpoint = 120
	doubleBreakpoint = 80

	sidebarMinWidth = 24

	statusBarHeight = 1
	// topBarHeight covers the workspace line plus its rule.
	topBarHeight = 2
	// separatorSize is the rule drawn between neighbouring panes.
	separatorSize = 1
)

// PanelSize is the outer box size of a panel, borders included.
type PanelSize struct {
	Width  int
	Height int
}

// PanelSizes leaves a panel zero-sized when it is not visible in the current
// mode.
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
	// splitDelta moves the request/response boundary: + widens request
	// in columns mode, + grows the request block in stacked mode.
	splitDelta int
	// stacked requests the stacked layout when the terminal is wide enough.
	stacked bool
	// sidebarWidth is the configured preferred width; 0 uses width/4.
	sidebarWidth int
}

// minPaneHeight keeps a stacked block usable.
const minPaneHeight = 6

// sidebarFor computes the tree width from config, user resizing, and limits.
func sidebarFor(width int, opts layoutOptions) int {
	preferred := width / 4
	if opts.sidebarWidth > 0 {
		preferred = opts.sidebarWidth
	}
	return clamp(preferred+opts.sidebarDelta, sidebarMinWidth, width/2)
}

func layout(width, height int, opts layoutOptions) PanelSizes {
	if width <= 0 || height <= statusBarHeight+topBarHeight {
		return PanelSizes{Mode: ModeSingle}
	}

	paneHeight := height - statusBarHeight - topBarHeight

	switch {
	case opts.stacked && width >= doubleBreakpoint && paneHeight >= 2*minPaneHeight+separatorSize:
		sidebar, sideRule := 0, 0
		if opts.sidebarVisible {
			sidebar = sidebarFor(width, opts)
			sideRule = separatorSize
		}
		main := width - sidebar - sideRule

		// One row goes to the rule between the stacked blocks.
		stack := paneHeight - separatorSize
		requestHeight := clamp(stack/2+opts.splitDelta, minPaneHeight, stack-minPaneHeight)
		return PanelSizes{
			Mode:        ModeStacked,
			Collections: PanelSize{sidebar, paneHeight},
			Request:     PanelSize{main, requestHeight},
			Response:    PanelSize{main, stack - requestHeight},
		}

	case width >= tripleBreakpoint:
		sidebar := sidebarFor(width, opts)
		avail := width - sidebar - 2*separatorSize
		request := clamp(avail/2+opts.splitDelta, minPaneWidth, avail-minPaneWidth)
		return PanelSizes{
			Mode:        ModeTriple,
			Collections: PanelSize{sidebar, paneHeight},
			Request:     PanelSize{request, paneHeight},
			Response:    PanelSize{avail - request, paneHeight},
		}

	case width >= doubleBreakpoint:
		if opts.sidebarVisible {
			sidebar := sidebarFor(width, opts)
			main := width - sidebar - separatorSize
			return PanelSizes{
				Mode:        ModeDouble,
				Collections: PanelSize{sidebar, paneHeight},
				Request:     PanelSize{main, paneHeight},
				Response:    PanelSize{main, paneHeight},
			}
		}
		avail := width - separatorSize
		request := clamp(avail/2+opts.splitDelta, minPaneWidth, avail-minPaneWidth)
		return PanelSizes{
			Mode:     ModeDouble,
			Request:  PanelSize{request, paneHeight},
			Response: PanelSize{avail - request, paneHeight},
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
