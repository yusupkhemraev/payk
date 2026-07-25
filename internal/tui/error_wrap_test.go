package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestResponseErrorWraps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "collections", "api", "broken.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	url := "{{first_missing_variable}}/{{second_missing_variable}}/" +
		"{{third_missing_variable}}/{{fourth_missing_variable}}"
	content := "name: broken\nmethod: GET\nurl: \"" + url + "\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 120, Height: 34})
	m = runCmds(m, m.Init())
	m = typeString(m, "j")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})

	// The full error is longer than the response pane: without wrapping the
	// last variable name would be truncated away.
	view := plainView(m)
	collapsed := collapseView(view)
	for _, want := range []string{"first_missing_variable", "fourth_missing_variable", "activeenvironment"} {
		if !strings.Contains(collapsed, want) {
			t.Errorf("wrapped error missing %q:\n%s", want, view)
		}
	}

	// The message log holds the error too.
	m = typeString(m, "m")
	if !strings.Contains(collapseView(plainView(m)), "fourth_missing_variable") {
		t.Errorf("message log missing the send error:\n%s", plainView(m))
	}
}
