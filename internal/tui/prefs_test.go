package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestLayoutCommandSwitchesAndPersists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := fixtureWorkspace(t)

	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m = runCmds(m, m.Init())

	if m.(Model).sizes.Mode != ModeStacked {
		t.Fatalf("default layout should be stacked, got %v", m.(Model).sizes.Mode)
	}

	m = runCommand(m, "layout columns")
	if got := m.(Model).sizes.Mode; got != ModeTriple {
		t.Errorf("layout should switch to columns, got %v", got)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "layout: columns") {
		t.Errorf("layout not persisted:\n%s", data)
	}
}

func TestThemeAndIconCommands(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := fixtureWorkspace(t)

	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m = runCmds(m, m.Init())

	m = runCommand(m, "theme latte")
	if got := m.(Model).theme.Flavor.Name(); got != "latte" {
		t.Errorf("theme = %q, want latte", got)
	}

	// Nerd icons put a glyph in front of folders; unicode uses arrows.
	m = runCommand(m, "icons none")
	if got := m.(Model).theme.Icons.Search; got != "" {
		t.Errorf("icons none should drop the search glyph, got %q", got)
	}

	m = runCommand(m, "icons unicode")
	if got := m.(Model).theme.Icons.Folder; got != "▸" {
		t.Errorf("unicode folder icon = %q", got)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "theme: latte") || !strings.Contains(content, "icons: unicode") {
		t.Errorf("preferences not persisted:\n%s", content)
	}
}

func TestUnknownPrefValueIsRejected(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var m tea.Model = New(testConfig(t))
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m = runCmds(m, m.Init())

	m = runCommand(m, "theme neon")
	if !strings.Contains(plainView(m), "unknown value") {
		t.Errorf("bad value should be reported:\n%s", plainView(m))
	}
	if got := m.(Model).theme.Flavor.Name(); got != "mocha" {
		t.Errorf("theme should stay mocha, got %q", got)
	}
}

func TestConfigFileDrivesStartupLayout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := workspaceWithConfig(t, "layout: columns\nicons: nerd\ntheme: frappe\n")

	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m = runCmds(m, m.Init())

	model := m.(Model)
	if model.sizes.Mode != ModeTriple {
		t.Errorf("config layout not applied: %v", model.sizes.Mode)
	}
	if model.theme.Flavor.Name() != "frappe" {
		t.Errorf("config theme not applied: %q", model.theme.Flavor.Name())
	}
	if model.theme.Icons.Folder == "▸" {
		t.Error("config icons not applied: still unicode")
	}
}

func TestStatusCachePersistsAndShowsInTree(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := fixtureWorkspace(t)

	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m = runCmds(m, m.Init())

	// Open ping and record a response for it.
	m = typeString(m, "G")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = stepMsg(m, responseReceivedMsg{label: "GET ping", err: errNoServer{}})

	if !strings.Contains(plainView(m), "ERR") {
		t.Errorf("failed send should show ERR in the tree:\n%s", plainView(m))
	}

	data, err := os.ReadFile(filepath.Join(dir, ".cache", "status.json"))
	if err != nil {
		t.Fatalf("status cache not written: %v", err)
	}
	if !strings.Contains(string(data), "api/ping") {
		t.Errorf("cache should key by request path:\n%s", data)
	}
	// The cache directory must not end up in commits.
	if _, err := os.Stat(filepath.Join(dir, ".cache", ".gitignore")); err != nil {
		t.Errorf("cache dir should carry a .gitignore: %v", err)
	}
}

type errNoServer struct{}

func (errNoServer) Error() string { return "connection refused" }
