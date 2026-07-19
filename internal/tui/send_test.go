package tui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"
)

// serverWorkspace builds a workspace with one request pointing at srv.
func serverWorkspace(t *testing.T, url string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "collections", "api", "hello.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "name: hello\nmethod: GET\nurl: " + url + "/hello\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSendRequestShowsResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Marker", "payk-test")
		_, _ = w.Write([]byte(`{"greeting": "hello from server"}`))
	}))
	t.Cleanup(srv.Close)

	cfg := Config{WorkspaceDir: serverWorkspace(t, srv.URL)}
	tm := teatest.NewTestModel(t, New(cfg), teatest.WithInitialTermSize(140, 40))
	waitForOutput(t, tm, "hello")

	// Open the request and send it with space.
	tm.Send(keyPress('j'))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "/hello")
	tm.Send(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})

	// Response body (pretty-printed JSON) and status appear.
	waitForOutput(t, tm, "200 OK", "greeting", "hello from server")

	// Focus the response pane (two hops right), then check the headers
	// and timings tabs.
	tm.Send(keyPress('l'))
	tm.Send(keyPress('l'))
	tm.Send(keyPress(']'))
	waitForOutput(t, tm, "X-Marker", "payk-test")
	tm.Send(keyPress(']'))
	waitForOutput(t, tm, "TTFB", "Total", "Size")

	tm.Send(keyPress('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestEditURLInInsertMode(t *testing.T) {
	cfg := Config{WorkspaceDir: fixtureWorkspace(t)}
	tm := teatest.NewTestModel(t, New(cfg), teatest.WithInitialTermSize(140, 40))
	waitForOutput(t, tm, "ping")

	// Open ping (last row), focus the editor, go to the url row.
	tm.Send(keyPress('G'))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "https://example.com/ping")
	tm.Send(keyPress('l'))
	tm.Send(keyPress('j'))

	// Insert mode: append a query string; keys like q must be typed, not quit.
	tm.Send(keyPress('i'))
	tm.Type("?q=1")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	waitForOutput(t, tm, "https://example.com/ping?q=1")

	// Method cycling on the method row: GET -> POST.
	tm.Send(keyPress('k'))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "POST")

	tm.Send(keyPress('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
