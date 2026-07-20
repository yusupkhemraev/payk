package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/httpc"
)

func TestResponseHistory(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_, _ = w.Write([]byte(`{"answer": "first-response-body"}`))
			return
		}
		_, _ = w.Write([]byte(`{"answer": "second-response-body"}`))
	}))
	t.Cleanup(srv.Close)

	var m tea.Model = New(Config{WorkspaceDir: serverWorkspace(t, srv.URL)})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())
	m = typeString(m, "j")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	send := func(label string) {
		model := m.(Model)
		resp, err := httpc.Send(context.Background(), model.request.CurrentRequest())
		if err != nil {
			t.Fatal(err)
		}
		m = stepMsg(m, responseReceivedMsg{label: label, resp: resp})
	}
	send("GET hello")
	send("GET hello again")

	// The viewer shows the latest response.
	if !strings.Contains(plainView(m), "second-response-body") {
		t.Fatalf("latest response should be shown:\n%s", plainView(m))
	}

	// The History tab lists both entries, newest first and selected.
	m = typeString(m, "ll")
	m = typeString(m, "]]]")
	view := plainView(m)
	if !strings.Contains(view, "History (2)") {
		t.Fatalf("history tab count missing:\n%s", view)
	}
	for _, want := range []string{"GET hello", "GET hello again", "200 OK"} {
		if !strings.Contains(view, want) {
			t.Errorf("history missing %q:\n%s", want, view)
		}
	}

	// Select the older entry: its body loads back into the viewer.
	m = typeString(m, "j")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	view = plainView(m)
	if !strings.Contains(view, "first-response-body") {
		t.Errorf("older response should load from history:\n%s", view)
	}
	if strings.Contains(view, "second-response-body") {
		t.Errorf("viewer should show only the selected response:\n%s", view)
	}
}
