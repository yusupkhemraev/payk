package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/httpc"
)

func TestChromeStyles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tests := []struct {
		chrome   string
		wants    []string
		rejects  []string
		describe string
	}{
		{
			chrome:   "boxed",
			wants:    []string{"╭", "─ Collections", "╰"},
			describe: "title sits in the top border",
		},
		{
			chrome:   "classic",
			wants:    []string{"╭", "│ Collections"},
			describe: "title gets its own row inside the border",
		},
		{
			chrome:   "plain",
			wants:    []string{"▎ Collections"},
			rejects:  []string{"╭"},
			describe: "no borders, just a focus bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.chrome, func(t *testing.T) {
			dir := workspaceWithConfig(t, "chrome: "+tt.chrome+"\n")
			var m tea.Model = New(Config{WorkspaceDir: dir})
			m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 40})
			m = runCmds(m, m.Init())

			view := plainView(m)
			for _, want := range tt.wants {
				if !strings.Contains(view, want) {
					t.Errorf("%s: missing %q\n%s", tt.describe, want, view)
				}
			}
			for _, reject := range tt.rejects {
				if strings.Contains(view, reject) {
					t.Errorf("%s: should not contain %q\n%s", tt.describe, reject, view)
				}
			}
		})
	}
}

// A theme change has to recolor a response that is already on screen.
func TestThemeSwitchRecolorsResponse(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name": "Ada"}`))
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

	before := m.(Model).View().Content
	m = runCommand(m, "theme latte")
	after := m.(Model).View().Content

	if before == after {
		t.Fatal("switching theme changed nothing on screen")
	}
	// The body keeps its text but must carry different color codes.
	if !strings.Contains(plainView(m), `"name": "Ada"`) {
		t.Errorf("response body lost after the theme switch:\n%s", plainView(m))
	}
	if strings.Contains(after, "38;2;205;214;244") {
		t.Error("response still carries Mocha foreground codes after switching to Latte")
	}
}

// A one-line JSON body is displayed pretty-printed; the file keeps its text
// until f rewrites it.
func TestRequestBodyDisplaysFormatted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	writeRequest(t, dir, "post.yaml",
		"name: post\nmethod: POST\nurl: https://example.com\n"+
			"body:\n  type: json\n  content: '{\"a\":1,\"b\":[2,3]}'\n")

	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m = runCmds(m, m.Init())
	m = typeString(m, "j")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	view := plainView(m)
	if !strings.Contains(view, `"a": 1`) {
		t.Errorf("one-line JSON should display pretty-printed:\n%s", view)
	}

	// The stored content is untouched until the user formats it.
	model := m.(Model)
	if got := model.request.CurrentRequest().Body.Content; strings.Contains(got, "\n") {
		t.Errorf("display formatting must not rewrite the request: %q", got)
	}
}

func writeRequest(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, "collections", "api", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
