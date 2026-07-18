package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/core"
	"github.com/yusupkhemraev/payk/internal/httpc"
)

// responseReceivedMsg carries the result of an HTTP send.
type responseReceivedMsg struct {
	resp *httpc.Response
	err  error
}

// sendRequestCmd performs the HTTP request; the context comes from the root
// model so esc can cancel an in-flight send.
func sendRequestCmd(ctx context.Context, req *core.Request) tea.Cmd {
	return func() tea.Msg {
		resp, err := httpc.Send(ctx, req)
		return responseReceivedMsg{resp: resp, err: err}
	}
}
