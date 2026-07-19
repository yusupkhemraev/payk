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

func TestRenameRequestUpdatesTreeAndDisk(t *testing.T) {
	dir := fixtureWorkspace(t)
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	// Walk to ping (collection row, collapsed users folder, ping).
	m = typeString(m, "G")
	m = typeString(m, "r")
	view := plainView(m)
	if !strings.Contains(view, "rename: ping") {
		t.Fatalf("rename input should be prefilled:\n%s", view)
	}

	// Replace the name entirely.
	m = stepMsg(m, tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	m = typeString(m, "healthcheck")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	view = plainView(m)
	if !strings.Contains(view, "healthcheck") {
		t.Errorf("tree should show the new name:\n%s", view)
	}
	if _, err := os.Stat(filepath.Join(dir, "collections", "api", "healthcheck.yaml")); err != nil {
		t.Errorf("renamed file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "collections", "api", "ping.yaml")); !os.IsNotExist(err) {
		t.Errorf("old file should be gone, stat err = %v", err)
	}
}

func TestRenameFolderMovesDirectory(t *testing.T) {
	dir := fixtureWorkspace(t)
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	// Row 1 is the collapsed users folder.
	m = typeString(m, "j")
	m = typeString(m, "r")
	m = stepMsg(m, tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	m = typeString(m, "people")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if !strings.Contains(plainView(m), "people") {
		t.Errorf("tree should show renamed folder:\n%s", plainView(m))
	}
	if _, err := os.Stat(filepath.Join(dir, "collections", "api", "people", "list-users.yaml")); err != nil {
		t.Errorf("folder contents should move: %v", err)
	}
}

func TestZoomTogglesFullscreenPane(t *testing.T) {
	var m tea.Model = New(testConfig(t))
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	// Zoom the request pane: other panels disappear from the view.
	m = typeString(m, "l")
	m = typeString(m, "z")
	view := plainView(m)
	if strings.Contains(view, "Collections") || strings.Contains(view, "Response") {
		t.Errorf("zoomed view should show only the focused pane:\n%s", view)
	}
	if !strings.Contains(view, "Request") || !strings.Contains(view, "zoom") {
		t.Errorf("zoomed pane and status flag missing:\n%s", view)
	}

	// h moves focus while zoomed; z toggles back to the split view.
	m = typeString(m, "h")
	if !strings.Contains(plainView(m), "Collections") {
		t.Errorf("focus switch while zoomed should show collections")
	}
	m = typeString(m, "z")
	view = plainView(m)
	if !strings.Contains(view, "Request") || !strings.Contains(view, "Response") {
		t.Errorf("unzoom should restore the split view:\n%s", view)
	}
}

func TestRawBodyToggle(t *testing.T) {
	minified := `{"a":1,"b":[2,3]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(minified))
	}))
	t.Cleanup(srv.Close)

	var m tea.Model = New(Config{WorkspaceDir: serverWorkspace(t, srv.URL)})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
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

	// Pretty by default: indented, so the minified form is absent.
	view := plainView(m)
	if strings.Contains(view, minified) {
		t.Fatalf("body should be pretty-printed by default:\n%s", view)
	}

	m = typeString(m, "r")
	view = plainView(m)
	if !strings.Contains(view, minified) {
		t.Errorf("raw mode should show original bytes:\n%s", view)
	}
	if !strings.Contains(view, "raw") {
		t.Errorf("raw flag missing from indicator:\n%s", view)
	}
}

func TestFormatJSONBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "collections", "api", "post.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "name: post\nmethod: POST\nurl: https://example.com\nbody:\n  type: json\n  content: '{\"a\":1,\"b\":2}'\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())
	m = typeString(m, "j")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	// Focus editor, go to Body tab, format.
	m = typeString(m, "l")
	m = typeString(m, "]]]")
	m = typeString(m, "f")

	view := plainView(m)
	if !strings.Contains(view, `"a": 1`) {
		t.Errorf("body should be pretty-printed:\n%s", view)
	}
	if !strings.Contains(view, "body formatted") {
		t.Errorf("status note missing:\n%s", view)
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := expandHome("~/dev/api"); got != filepath.Join(home, "dev/api") {
		t.Errorf("expandHome = %q", got)
	}
	if got := expandHome("/abs/path"); got != "/abs/path" {
		t.Errorf("absolute path must pass through, got %q", got)
	}
	if got := expandHome("~user/x"); got != "~user/x" {
		t.Errorf("~user form must pass through, got %q", got)
	}
}
