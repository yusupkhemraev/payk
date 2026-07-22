package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

const specV1 = `{"openapi": "3.1.0", "info": {"title": "Shop"}, "paths": {
  "/items": {"get": {"summary": "List items", "tags": ["items"]}},
  "/legacy": {"get": {"summary": "Old route", "tags": ["items"]}}}}`

const specV2 = `{"openapi": "3.1.0", "info": {"title": "Shop"}, "paths": {
  "/items": {"get": {"summary": "List items", "tags": ["items"]}},
  "/items/new": {"post": {"summary": "Brand new route", "tags": ["items"]}}}}`

func TestReimportPicksUpChangedSpec(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "openapi.json")
	if err := os.WriteFile(spec, []byte(specV1), 0o644); err != nil {
		t.Fatal(err)
	}

	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	m = runCommand(m, "import "+spec)
	// Expand the items folder.
	m = typeString(m, "j")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	view := plainView(m)
	for _, want := range []string{"List items", "Old route"} {
		if !strings.Contains(view, want) {
			t.Fatalf("initial import missing %q:\n%s", want, view)
		}
	}

	// The spec changes: one route removed, one added.
	if err := os.WriteFile(spec, []byte(specV2), 0o644); err != nil {
		t.Fatal(err)
	}
	m = runCommand(m, "reimport shop")
	m = typeString(m, "gg")
	m = typeString(m, "j")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	view = plainView(m)
	if !strings.Contains(view, "Brand new route") {
		t.Errorf("new route missing after reimport:\n%s", view)
	}
	if strings.Contains(view, "Old route") {
		t.Errorf("removed route should be gone after reimport:\n%s", view)
	}
	if _, err := os.Stat(filepath.Join(dir, "collections", "shop", "items", "old-route.yaml")); !os.IsNotExist(err) {
		t.Errorf("stale file should be deleted, stat err = %v", err)
	}
}

func TestReimportAllWithoutArgs(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "openapi.json")
	if err := os.WriteFile(spec, []byte(specV1), 0o644); err != nil {
		t.Fatal(err)
	}

	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())
	m = runCommand(m, "import "+spec)

	if err := os.WriteFile(spec, []byte(specV2), 0o644); err != nil {
		t.Fatal(err)
	}
	m = runCommand(m, "reimport")
	m = typeString(m, "gg")
	m = typeString(m, "j")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(plainView(m), "Brand new route") {
		t.Errorf("bare :reimport should refresh sourced collections:\n%s", plainView(m))
	}
}

func TestCurlImportsAppendInsteadOfReplacing(t *testing.T) {
	dir := t.TempDir()
	var m tea.Model = New(Config{WorkspaceDir: dir})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())

	m = stepMsg(m, tea.PasteMsg{Content: "curl https://example.com/first"})
	m = typeString(m, "y")
	m = stepMsg(m, tea.PasteMsg{Content: "curl https://example.com/second"})
	_ = typeString(m, "y")

	entries, err := os.ReadDir(filepath.Join(dir, "collections", "imported"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("curl imports must accumulate, got %d files", len(entries))
	}
}

func TestCreateRequestWithoutAnyWorkspace(t *testing.T) {
	work := t.TempDir()
	t.Chdir(work)
	t.Setenv("HOME", t.TempDir())

	// No WorkspaceDir, no .payk anywhere up the tree, empty HOME: the
	// tree shows the hint and a still works.
	var m tea.Model = New(Config{})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m = runCmds(m, m.Init())
	if !strings.Contains(plainView(m), "no requests yet") {
		t.Fatalf("no-workspace hint missing:\n%s", plainView(m))
	}

	m = typeString(m, "a")
	m = typeString(m, "first route")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if _, err := os.Stat(filepath.Join(work, ".payk", "collections", "api", "first-route.yaml")); err != nil {
		t.Fatalf("workspace should be created on the fly: %v", err)
	}
	if !strings.Contains(plainView(m), "first route") {
		t.Errorf("tree should show the created request:\n%s", plainView(m))
	}
}
