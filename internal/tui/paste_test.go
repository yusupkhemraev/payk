package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const pastedCurl = `curl 'https://api.example.com/v1/items?active=1' \
  -H 'accept: application/json' \
  -H 'authorization: Bearer tok' \
  --data-raw '{"kind": "book"}' \
  --compressed`

func TestPasteCurlPromptAndImport(t *testing.T) {
	dir := t.TempDir()
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	// Paste triggers the inline prompt without touching the tree yet.
	m = stepMsg(m, tea.PasteMsg{Content: pastedCurl})
	view := ansi.Strip(m.(Model).View().Content)
	if !strings.Contains(view, "import as request? (y/n)") {
		t.Fatalf("import prompt not shown:\n%s", view)
	}

	// n dismisses without importing.
	m = stepMsg(m, tea.KeyPressMsg{Code: 'n', Text: "n"})
	if entries, _ := os.ReadDir(filepath.Join(dir, "collections")); len(entries) != 0 {
		t.Fatal("declined import must not write anything")
	}

	// Paste again, accept with y: request lands in the tree and on disk.
	m = stepMsg(m, tea.PasteMsg{Content: pastedCurl})
	m = stepMsg(m, tea.KeyPressMsg{Code: 'y', Text: "y"})

	view = ansi.Strip(m.(Model).View().Content)
	if !strings.Contains(view, "imported POST /v1/items") {
		t.Errorf("status should confirm import:\n%s", view)
	}
	if !strings.Contains(view, "POST") || !strings.Contains(view, "/v1/items") {
		t.Errorf("tree should show the imported request:\n%s", view)
	}

	data, err := os.ReadFile(filepath.Join(dir, "collections", "imported", "post-v1items.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{"method: POST", "https://api.example.com/v1/items?active=1", `"kind": "book"`} {
		if !strings.Contains(content, want) {
			t.Errorf("saved file missing %q:\n%s", want, content)
		}
	}
}

func TestPasteNonCurlGoesToFocusedInput(t *testing.T) {
	cfg := Config{WorkspaceDir: fixtureWorkspace(t)}
	var m tea.Model = New(cfg)
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	// Open ping, focus editor, enter insert mode on the url row.
	m = stepMsg(m, tea.KeyPressMsg{Code: 'G', Text: "G"})
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = stepMsg(m, tea.KeyPressMsg{Code: 'l', Text: "l"})
	m = stepMsg(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = stepMsg(m, tea.KeyPressMsg{Code: 'i', Text: "i"})

	// Pasting even a genuine curl command in insert mode must go into the
	// input, not open the import prompt.
	m = stepMsg(m, tea.PasteMsg{Content: "curl https://pasted.example"})
	view := ansi.Strip(m.(Model).View().Content)
	if strings.Contains(view, "import as request?") {
		t.Fatal("import prompt must not open in insert mode")
	}
	// The frame truncates long lines, so match the joint where the pasted
	// text was appended to the existing URL.
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	view = ansi.Strip(m.(Model).View().Content)
	if !strings.Contains(view, "pingcurl https") {
		t.Errorf("pasted text should land in the url field:\n%s", view)
	}
}
