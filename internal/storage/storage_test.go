package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusupkhemraev/payk/internal/core"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadCollectionsBuildsSortedTree(t *testing.T) {
	ws := &Workspace{Dir: t.TempDir()}
	root := ws.CollectionsDir()

	writeFile(t, filepath.Join(root, "api", "users", "list-users.yaml"),
		"name: list users\nmethod: get\nurl: https://example.com/users\n")
	writeFile(t, filepath.Join(root, "api", "users", "create-user.yaml"),
		"name: create user\nmethod: POST\nurl: https://example.com/users\n")
	writeFile(t, filepath.Join(root, "api", "ping.yaml"),
		"method: GET\nurl: https://example.com/ping\n")
	writeFile(t, filepath.Join(root, "admin", "stats.yaml"),
		"name: stats\nmethod: GET\nurl: https://example.com/stats\n")
	// Non-YAML files and hidden directories must be ignored.
	writeFile(t, filepath.Join(root, "api", "notes.txt"), "ignore me")
	writeFile(t, filepath.Join(root, ".hidden", "x.yaml"), "url: ignored\n")

	cols, err := ws.LoadCollections()
	if err != nil {
		t.Fatal(err)
	}

	if len(cols) != 2 {
		t.Fatalf("got %d collections, want 2", len(cols))
	}
	if cols[0].Name != "admin" || cols[1].Name != "api" {
		t.Errorf("collections not sorted: %q, %q", cols[0].Name, cols[1].Name)
	}

	api := cols[1]
	if len(api.Folders) != 1 || api.Folders[0].Name != "users" {
		t.Fatalf("api folders = %+v, want one folder 'users'", api.Folders)
	}
	if len(api.Requests) != 1 || api.Requests[0].Name != "ping" {
		t.Errorf("request without name should fall back to file name, got %+v", api.Requests)
	}

	users := api.Folders[0]
	if len(users.Requests) != 2 {
		t.Fatalf("users has %d requests, want 2", len(users.Requests))
	}
	// Sorted by file name: create-user before list-users.
	if users.Requests[0].Name != "create user" || users.Requests[1].Name != "list users" {
		t.Errorf("requests not sorted by file name: %q, %q",
			users.Requests[0].Name, users.Requests[1].Name)
	}
	if users.Requests[1].Method != "GET" {
		t.Errorf("method not normalized to upper case: %q", users.Requests[1].Method)
	}
}

func TestLoadCollectionsMissingDirIsEmpty(t *testing.T) {
	ws := &Workspace{Dir: t.TempDir()}
	cols, err := ws.LoadCollections()
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 0 {
		t.Errorf("got %d collections, want 0", len(cols))
	}
}

func TestLoadCollectionsReportsBrokenFile(t *testing.T) {
	ws := &Workspace{Dir: t.TempDir()}
	writeFile(t, filepath.Join(ws.CollectionsDir(), "api", "bad.yaml"), "{{{not yaml")

	_, err := ws.LoadCollections()
	if err == nil {
		t.Fatal("want error for broken YAML")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("bad.yaml")) {
		t.Errorf("error should name the broken file, got: %v", err)
	}
}

func TestSaveRequestRoundTripAndStableBytes(t *testing.T) {
	ws := &Workspace{Dir: t.TempDir()}
	req := &core.Request{
		Name:   "Create User",
		Method: "POST",
		URL:    "https://example.com/users",
		Params: []core.KV{{Name: "verbose", Value: "1"}},
		Headers: []core.KV{
			{Name: "Content-Type", Value: "application/json"},
		},
		Body: core.Body{Type: "json", Content: `{"login": "x"}`},
		Auth: core.Auth{Type: core.AuthBearer, Token: "{{env:API_TOKEN}}"},
	}

	if err := ws.SaveRequest("api", []string{"users"}, req); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ws.CollectionsDir(), "api", "users", "create-user.yaml")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := ws.SaveRequest("api", []string{"users"}, req); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if !bytes.Equal(first, second) {
		t.Error("repeated saves must produce identical bytes")
	}

	cols, err := ws.LoadCollections()
	if err != nil {
		t.Fatal(err)
	}
	loaded := cols[0].Folders[0].Requests[0]
	if loaded.Name != req.Name || loaded.URL != req.URL ||
		loaded.Body != req.Body || loaded.Auth != req.Auth {
		t.Errorf("round trip mismatch:\nsaved  %+v\nloaded %+v", req, loaded)
	}
}

func TestSaveRequestOmitsEmptySections(t *testing.T) {
	ws := &Workspace{Dir: t.TempDir()}
	req := &core.Request{Name: "ping", Method: "GET", URL: "https://example.com/ping"}

	if err := ws.SaveRequest("api", nil, req); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(ws.CollectionsDir(), "api", "ping.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"params", "headers", "body", "auth"} {
		if bytes.Contains(data, []byte(key+":")) {
			t.Errorf("empty %q section should be omitted, file:\n%s", key, data)
		}
	}
}

func TestDiscoverWalksUpToProjectWorkspace(t *testing.T) {
	root := t.TempDir()
	wsDir := filepath.Join(root, ".payk")
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	ws, err := Discover(nested)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Dir != wsDir {
		t.Errorf("discovered %q, want %q", ws.Dir, wsDir)
	}
}

func TestDiscoverFallsBackToGlobalThenNotFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := Discover(t.TempDir()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	global := filepath.Join(home, ".config", "payk")
	if err := os.MkdirAll(global, 0o755); err != nil {
		t.Fatal(err)
	}
	ws, err := Discover(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ws.Dir != global {
		t.Errorf("discovered %q, want global %q", ws.Dir, global)
	}
}

func TestEnvironmentsRoundTrip(t *testing.T) {
	ws := &Workspace{Dir: t.TempDir()}

	envs, err := ws.LoadEnvironments()
	if err != nil {
		t.Fatal(err)
	}
	if envs.Active != "" || len(envs.Environments) != 0 {
		t.Errorf("missing file should load as empty config, got %+v", envs)
	}

	saved := &core.Environments{
		Active: "local",
		Environments: []core.Environment{
			{Name: "local", Vars: map[string]string{"base_url": "http://localhost:8000", "api_token": "{{env:API_TOKEN}}"}},
			{Name: "staging", Vars: map[string]string{"base_url": "https://staging.example.com"}},
		},
	}
	if err := ws.SaveEnvironments(saved); err != nil {
		t.Fatal(err)
	}

	first, _ := os.ReadFile(ws.EnvironmentsFile())
	if err := ws.SaveEnvironments(saved); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(ws.EnvironmentsFile())
	if !bytes.Equal(first, second) {
		t.Error("repeated saves must produce identical bytes")
	}

	loaded, err := ws.LoadEnvironments()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Active != "local" || len(loaded.Environments) != 2 {
		t.Fatalf("round trip mismatch: %+v", loaded)
	}
	if loaded.ActiveEnv().Vars["base_url"] != "http://localhost:8000" {
		t.Errorf("active env vars lost: %+v", loaded.ActiveEnv())
	}
}

func TestSlug(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Create User", "create-user"},
		{"  List  Users  ", "list--users"},
		{"GET /users/{id}", "get-usersid"},
		{"тест", "тест"},
		{"Пользователи API", "пользователи-api"},
		{"", "request"},
	}
	for _, tt := range tests {
		if got := slug(tt.in); got != tt.want {
			t.Errorf("slug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
