package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func runCommand(m tea.Model, command string) tea.Model {
	m = typeString(m, ":")
	m = stepMsg(m, tea.PasteMsg{Content: command})
	return stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
}

func TestEnvCreateSetUnset(t *testing.T) {
	dir := t.TempDir()
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	// No environments yet: :env lists the hint.
	m = runCommand(m, "env")
	if !strings.Contains(plainView(m), "no environments") {
		t.Fatalf("empty env list hint missing:\n%s", plainView(m))
	}

	// :env new creates, activates, and persists.
	m = runCommand(m, "env new dev")
	if !strings.Contains(plainView(m), "env:dev") {
		t.Fatalf("created env should be active:\n%s", plainView(m))
	}

	// :set adds a variable to the active environment.
	m = runCommand(m, "set base_url http://localhost:9000")
	data, err := os.ReadFile(filepath.Join(dir, "environments.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{"active: dev", "base_url: http://localhost:9000"} {
		if !strings.Contains(content, want) {
			t.Errorf("environments.yaml missing %q:\n%s", want, content)
		}
	}

	// name=value form works too.
	m = runCommand(m, "set token=secret value")
	data, _ = os.ReadFile(filepath.Join(dir, "environments.yaml"))
	if !strings.Contains(string(data), "token: secret value") {
		t.Errorf("name=value form not saved:\n%s", data)
	}

	// :env lists with the active one marked.
	m = runCommand(m, "env")
	if !strings.Contains(plainView(m), "*dev") {
		t.Errorf("list should mark active env:\n%s", plainView(m))
	}

	// :unset removes the variable.
	_ = runCommand(m, "unset token")
	data, _ = os.ReadFile(filepath.Join(dir, "environments.yaml"))
	if strings.Contains(string(data), "token:") {
		t.Errorf("token should be removed:\n%s", data)
	}
}

func TestSetBootstrapsDefaultEnvironment(t *testing.T) {
	dir := t.TempDir()
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	_ = runCommand(m, "set base_url http://x")
	data, err := os.ReadFile(filepath.Join(dir, "environments.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "active: default") || !strings.Contains(content, "base_url: http://x") {
		t.Errorf(":set without envs should bootstrap a default one:\n%s", content)
	}
}

func TestSetFeedsCompletion(t *testing.T) {
	dir := fixtureWorkspace(t)
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	m = runCommand(m, "env new dev")
	m = runCommand(m, "set my_special_var 42")

	// Open ping, edit the url, type {{ — the new var must be suggested.
	m = typeString(m, "G")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = typeString(m, "l")
	m = typeString(m, "j")
	m = typeString(m, "i")
	m = typeString(m, "{{my_sp")
	if !strings.Contains(plainView(m), "{{my_special_var}}") {
		t.Errorf("fresh variable should appear in completion:\n%s", plainView(m))
	}
}
