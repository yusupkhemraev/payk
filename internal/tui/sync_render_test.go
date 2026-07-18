package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yusupkhemraev/payk/internal/httpc"
)

// runCmds executes non-blocking commands synchronously, feeding results back
// into Update. Commands that need a running Program (ticks) must not enter.
func runCmds(m tea.Model, cmd tea.Cmd) tea.Model {
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = runCmds(m, c)
		}
		return m
	}
	next, nextCmd := m.Update(msg)
	return runCmds(next, nextCmd)
}

func stepMsg(m tea.Model, msg tea.Msg) tea.Model {
	next, cmd := m.Update(msg)
	return runCmds(next, cmd)
}

func TestResponseRenderSync(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"users": [{"id": 1, "name": "Ada"}], "total": 1}`))
	}))
	t.Cleanup(srv.Close)

	var m tea.Model = New(Config{WorkspaceDir: serverWorkspace(t, srv.URL)})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 120, Height: 24})
	m = runCmds(m, m.Init())
	m = stepMsg(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	// Bypass the spinner tick (needs a running Program): send directly and
	// feed the result message in, as the send command would.
	model := m.(Model)
	resp, err := httpc.Send(context.Background(), model.request.CurrentRequest())
	m = stepMsg(m, responseReceivedMsg{resp: resp, err: err})

	body := ansi.Strip(m.(Model).View().Content)
	for _, want := range []string{"200 OK", `"name": "Ada"`, `"total": 1`} {
		if !strings.Contains(body, want) {
			t.Errorf("body view missing %q:\n%s", want, body)
		}
	}

	m = stepMsg(m, tea.KeyPressMsg{Code: 'l', Text: "l"})
	m = stepMsg(m, tea.KeyPressMsg{Code: 'l', Text: "l"})
	m = stepMsg(m, tea.KeyPressMsg{Code: ']', Text: "]"})
	m = stepMsg(m, tea.KeyPressMsg{Code: ']', Text: "]"})

	timings := ansi.Strip(m.(Model).View().Content)
	for _, want := range []string{"TTFB", "Total", "TCP connect", "Status       200 OK"} {
		if !strings.Contains(timings, want) {
			t.Errorf("timings view missing %q:\n%s", want, timings)
		}
	}
}
