package tui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"
)

// envWorkspace builds a workspace with a templated request and two
// environments; local points at the given base URL.
func envWorkspace(t *testing.T, baseURL string) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"collections/api/hello.yaml": "name: hello\nmethod: GET\nurl: \"{{base_url}}/hello\"\n",
		"environments.yaml": "active: local\nenvironments:\n" +
			"  - name: local\n    vars:\n      base_url: " + baseURL + "\n" +
			"  - name: staging\n    vars:\n      base_url: https://staging.invalid\n",
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

func TestSendSubstitutesEnvironmentVars(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"via": "template"}`))
	}))
	t.Cleanup(srv.Close)

	cfg := Config{WorkspaceDir: envWorkspace(t, srv.URL)}
	tm := teatest.NewTestModel(t, New(cfg), teatest.WithInitialTermSize(140, 40))
	waitForOutput(t, tm, "hello", "env:local")

	tm.Send(keyPress('j'))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "{{base_url}}/hello")
	tm.Send(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	waitForOutput(t, tm, "200 OK", "via", "template")

	tm.Send(keyPress('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestSendReportsMissingVariables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "collections", "api", "broken.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "name: broken\nmethod: GET\nurl: \"{{nope}}/x\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tm := teatest.NewTestModel(t, New(Config{WorkspaceDir: dir}), teatest.WithInitialTermSize(140, 40))
	waitForOutput(t, tm, "broken")

	tm.Send(keyPress('j'))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "{{nope}}/x")
	// The frame truncates long lines, so match only the message start.
	tm.Send(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	waitForOutput(t, tm, "undefined variables: nope")

	tm.Send(keyPress('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func typeString(m tea.Model, s string) tea.Model {
	for _, r := range s {
		m = stepMsg(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m
}

func TestEnvCommandSwitchesAndPersists(t *testing.T) {
	dir := envWorkspace(t, "http://localhost:1")
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	m = typeString(m, ":")
	m = typeString(m, "env staging")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	bar := ansi.Strip(m.(Model).View().Content)
	if !strings.Contains(bar, "env:staging") {
		t.Errorf("status bar should show new environment:\n%s", bar)
	}

	data, err := os.ReadFile(filepath.Join(dir, "environments.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "active: staging") {
		t.Errorf("active environment not persisted:\n%s", data)
	}

	// Unknown environment: error status, active unchanged.
	m = typeString(m, ":")
	m = typeString(m, "env nope")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	bar = ansi.Strip(m.(Model).View().Content)
	if !strings.Contains(bar, "no such environment: nope") {
		t.Errorf("missing error for unknown env:\n%s", bar)
	}
}

func TestWriteCommandSavesEditedRequest(t *testing.T) {
	dir := envWorkspace(t, "http://localhost:1")
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	// Open hello, focus editor, edit the URL in insert mode.
	m = stepMsg(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = typeString(m, "l")
	m = stepMsg(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = typeString(m, "i")
	m = typeString(m, "?edited=1")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	m = typeString(m, ":w")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	data, err := os.ReadFile(filepath.Join(dir, "collections", "api", "hello.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "{{base_url}}/hello?edited=1") {
		t.Errorf("edited URL not saved, file:\n%s", content)
	}
	if !strings.Contains(content, "{{base_url}}") {
		t.Errorf("placeholders must stay unresolved on disk:\n%s", content)
	}

	bar := ansi.Strip(m.(Model).View().Content)
	if !strings.Contains(bar, "saved hello") {
		t.Errorf("status should confirm save:\n%s", bar)
	}
}
