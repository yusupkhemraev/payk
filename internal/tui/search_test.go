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

func plainView(m tea.Model) string {
	return ansi.Strip(m.(Model).View().Content)
}

func TestURLCompletionFromEnvironment(t *testing.T) {
	dir := envWorkspace(t, "http://localhost:1")
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	// Open hello, focus editor, edit url, wipe it and type "{{".
	m = stepMsg(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = typeString(m, "l")
	m = stepMsg(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = typeString(m, "i")
	m = stepMsg(m, tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}) // clear line
	m = typeString(m, "{{ba")

	view := plainView(m)
	if !strings.Contains(view, "{{base_url}}") {
		t.Fatalf("suggestion line should offer base_url:\n%s", view)
	}

	// Tab accepts the completion into the input.
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	model := m.(Model)
	if got := model.request.CurrentRequest().URL; got != "{{base_url}}" {
		t.Errorf("completed URL not committed, got %q", got)
	}
}

func TestEnvPrefixCompletionUsesProcessEnv(t *testing.T) {
	t.Setenv("PAYK_TEST_TOKEN", "x")
	dir := envWorkspace(t, "http://localhost:1")
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	m = stepMsg(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = typeString(m, "l")
	m = stepMsg(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = typeString(m, "i")
	m = typeString(m, "{{env:PAYK_TEST")

	view := plainView(m)
	if !strings.Contains(view, "{{env:PAYK_TEST_TOKEN}}") {
		t.Fatalf("process env completion missing:\n%s", view)
	}
}

func TestCollectionsTreeSearch(t *testing.T) {
	cfg := Config{WorkspaceDir: fixtureWorkspace(t)}
	var m tea.Model = New(cfg)
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	// Filter as you type: only "create user" matches.
	m = typeString(m, "/")
	m = typeString(m, "create")
	view := plainView(m)
	if !strings.Contains(view, "create user") {
		t.Fatalf("match should stay visible:\n%s", view)
	}
	if strings.Contains(view, "list users") || strings.Contains(view, "GET    ping") {
		t.Errorf("non-matches should be filtered out:\n%s", view)
	}
	// The narrow sidebar truncates the full path; the prefix is enough.
	if !strings.Contains(view, "api/") {
		t.Errorf("match should show its tree path:\n%s", view)
	}

	// Enter jumps to the match in the restored tree; enter again opens it.
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	view = plainView(m)
	if !strings.Contains(view, "GET    ping") {
		t.Errorf("full tree should be restored after enter:\n%s", view)
	}
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	view = plainView(m)
	if !strings.Contains(view, "https://example.com/users") {
		t.Errorf("selected match should open in the editor:\n%s", view)
	}

	// Esc cancels a search without jumping.
	m = typeString(m, "/")
	m = typeString(m, "zzz-nothing")
	view = plainView(m)
	if !strings.Contains(view, "no matches") {
		t.Errorf("empty result hint missing:\n%s", view)
	}
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	view = plainView(m)
	if !strings.Contains(view, "GET    ping") {
		t.Errorf("tree should be restored after esc:\n%s", view)
	}
}

func TestResponseBodySearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"first": "alpha", "second": "beta", "third": "alpha again"}`))
	}))
	t.Cleanup(srv.Close)

	var m tea.Model = New(Config{WorkspaceDir: serverWorkspace(t, srv.URL)})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())
	m = stepMsg(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	model := m.(Model)
	resp, err := httpc.Send(context.Background(), model.request.CurrentRequest())
	if err != nil {
		t.Fatal(err)
	}
	m = stepMsg(m, responseReceivedMsg{resp: resp})

	// Focus response, search for "alpha": two matching lines.
	m = typeString(m, "ll")
	m = typeString(m, "/")
	m = typeString(m, "alpha")
	view := plainView(m)
	if !strings.Contains(view, "2 matches") {
		t.Fatalf("live match count missing:\n%s", view)
	}

	// Enter commits; n cycles 1/2 -> 2/2.
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	view = plainView(m)
	if !strings.Contains(view, "/alpha  1/2") {
		t.Errorf("committed search line missing:\n%s", view)
	}
	m = typeString(m, "n")
	view = plainView(m)
	if !strings.Contains(view, "/alpha  2/2") {
		t.Errorf("n should advance to the next match:\n%s", view)
	}
	m = typeString(m, "n")
	if !strings.Contains(plainView(m), "/alpha  1/2") {
		t.Errorf("n should wrap around")
	}
}
