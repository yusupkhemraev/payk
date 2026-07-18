package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"
)

// fixtureWorkspace writes a small .payk-style tree into a temp dir.
func fixtureWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"collections/api/users/list-users.yaml": "name: list users\nmethod: GET\nurl: https://example.com/users\n",
		"collections/api/users/create-user.yaml": "name: create user\nmethod: POST\nurl: https://example.com/users\n" +
			"headers:\n  - name: Content-Type\n    value: application/json\n",
		"collections/api/ping.yaml": "name: ping\nmethod: GET\nurl: https://example.com/ping\n",
	}
	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestBrowseTreeAndOpenRequest(t *testing.T) {
	cfg := Config{WorkspaceDir: fixtureWorkspace(t)}
	tm := teatest.NewTestModel(t, New(cfg), teatest.WithInitialTermSize(140, 40))

	// Tree loads: collection, folder, and requests are visible.
	waitForOutput(t, tm, "api", "users", "list users", "create user", "ping")

	// j moves to the folder row, enter collapses it.
	tm.Send(keyPress('j'))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "▸ users")

	// Expand back, then walk down to "create user" and open it: the
	// request editor must show its method and URL on the URL tab.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	tm.Send(keyPress('j'))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "https://example.com/users")

	// Switch focus to the editor and flip to the Headers tab.
	tm.Send(keyPress('l'))
	tm.Send(keyPress(']'))
	tm.Send(keyPress(']'))
	waitForOutput(t, tm, "Content-Type = application/json")

	tm.Send(keyPress('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestEmptyWorkspaceShowsHint(t *testing.T) {
	tm := teatest.NewTestModel(t, New(testConfig(t)), teatest.WithInitialTermSize(140, 40))
	waitForOutput(t, tm, "workspace is empty")
	tm.Send(keyPress('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
