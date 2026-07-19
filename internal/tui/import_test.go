package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestImportCommandLoadsOpenAPISpec(t *testing.T) {
	dir := t.TempDir()
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m = runCmds(m, m.Init())

	// Paste the long path into the command line instead of typing it —
	// also covers paste routing into the open cmdline.
	spec := filepath.Join("..", "importer", "openapi", "testdata", "petstore-3.1.yaml")
	m = typeString(m, ":")
	m = stepMsg(m, tea.PasteMsg{Content: "import " + spec})
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	view := plainView(m)
	for _, want := range []string{"swagger-petstore", "pets", "store"} {
		if !strings.Contains(view, want) {
			t.Errorf("tree missing %q after import:\n%s", want, view)
		}
	}

	// Folders start collapsed; walk to pets (row 2 after healthz) and
	// expand it to see its requests.
	m = stepMsg(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = stepMsg(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	view = plainView(m)
	for _, want := range []string{"List pets", "Create a pet"} {
		if !strings.Contains(view, want) {
			t.Errorf("expanded folder missing %q:\n%s", want, view)
		}
	}

	// Server URLs became environments in environments.yaml.
	data, err := os.ReadFile(filepath.Join(dir, "environments.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	envs := string(data)
	for _, want := range []string{"swagger-petstore", "https://petstore.example.com/v2", "swagger-petstore-2"} {
		if !strings.Contains(envs, want) {
			t.Errorf("environments.yaml missing %q:\n%s", want, envs)
		}
	}

	// The imported environment is switchable and feeds {{base_url}}.
	m = typeString(m, ":")
	m = typeString(m, "env swagger-petstore")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(plainView(m), "env:swagger-petstore") {
		t.Errorf("imported environment should be active:\n%s", plainView(m))
	}

	// Imported requests carry the {{base_url}} placeholder and example body.
	saved, err := os.ReadFile(filepath.Join(dir, "collections", "swagger-petstore", "pets", "create-a-pet.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(saved)
	for _, want := range []string{"{{base_url}}/pets", `"name": "doggie"`, "type: json"} {
		if !strings.Contains(content, want) {
			t.Errorf("saved request missing %q:\n%s", want, content)
		}
	}
}

func TestImportCommandRejectsUnknownInput(t *testing.T) {
	var m tea.Model = New(testConfig(t))
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	m = typeString(m, ":")
	m = typeString(m, "import nonsense-input")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if !strings.Contains(plainView(m), "no importer can handle") {
		t.Errorf("expected rejection status:\n%s", plainView(m))
	}
}
