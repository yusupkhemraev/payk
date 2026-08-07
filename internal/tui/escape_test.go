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

// esc must reach the focused pane; only an in-flight send takes it first.
func TestEscapeClearsCommittedSearch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"title": "delectus", "done": false}`))
	}))
	t.Cleanup(srv.Close)

	var m tea.Model = New(Config{WorkspaceDir: serverWorkspace(t, srv.URL)})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m = runCmds(m, m.Init())
	m = typeString(m, "j")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	model := m.(Model)
	resp, err := httpc.Send(context.Background(), model.request.CurrentRequest())
	if err != nil {
		t.Fatal(err)
	}
	m = stepMsg(m, responseReceivedMsg{resp: resp})

	m = typeString(m, "ll")
	m = typeString(m, "/")
	m = typeString(m, "title")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(plainView(m), "/title") {
		t.Fatalf("committed search should stay visible:\n%s", plainView(m))
	}

	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if strings.Contains(plainView(m), "/title") {
		t.Errorf("esc should clear the committed search:\n%s", plainView(m))
	}
}
