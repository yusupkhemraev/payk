package fastapi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fakeSpec = `{"openapi": "3.1.0", "info": {"title": "FastAPI", "version": "0.1.0"},
"paths": {"/users/": {"get": {"summary": "Read Users", "operationId": "read_users"}}}}`

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// fakeProject builds a FastAPI-looking project whose .venv python is a shell
// script; it logs its arguments and prints the canned spec.
func fakeProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "main.py"),
		"from fastapi import FastAPI\n\napp = FastAPI()\n", 0o644)
	script := "#!/bin/sh\necho \"$@\" > args.log\necho '" + fakeSpec + "'\n"
	writeFile(t, filepath.Join(dir, ".venv", "bin", "python"), script, 0o755)
	return dir
}

func TestImportRunsProjectInterpreter(t *testing.T) {
	dir := fakeProject(t)
	imp := New()

	col, err := imp.Import(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}

	// Generic "FastAPI" title is replaced by the project directory name.
	if col.Name != filepath.Base(dir) {
		t.Errorf("collection name = %q, want %q", col.Name, filepath.Base(dir))
	}
	if len(col.Folders) != 1 || col.Folders[0].Name != "users" {
		t.Fatalf("folders = %+v", col.Folders)
	}

	// The subprocess ran with cwd = project root and got the right -c code.
	args, err := os.ReadFile(filepath.Join(dir, "args.log"))
	if err != nil {
		t.Fatal("interpreter was not invoked with cwd = project root:", err)
	}
	for _, want := range []string{"import_module('app.main')", "getattr(m, 'app')", "import json, importlib"} {
		if !strings.Contains(string(args), want) {
			t.Errorf("interpreter args missing %q: %s", want, args)
		}
	}

	// FastAPI specs have no servers: the warning must pass through.
	if len(imp.Warnings()) != 1 || !strings.Contains(imp.Warnings()[0], "no servers") {
		t.Errorf("warnings = %v", imp.Warnings())
	}
}

func TestImportSurfacesStderrOnFailure(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.py"), "app = FastAPI()\n", 0o644)
	script := "#!/bin/sh\necho 'ModuleNotFoundError: No module named missing_dep' >&2\nexit 1\n"
	writeFile(t, filepath.Join(dir, ".venv", "bin", "python"), script, 0o755)

	_, err := New().Import(context.Background(), dir)
	if err == nil {
		t.Fatal("want error when the app fails to import")
	}
	for _, want := range []string{"main:app", "ModuleNotFoundError", "missing_dep"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should surface %q, got:\n%v", want, err)
		}
	}
}

func TestPyprojectEntrypointWinsOverHeuristic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"),
		"[tool.poetry]\nname = \"x\"\n\n[tool.payk]\nentrypoint = \"backend.api:application\"\n", 0o644)
	writeFile(t, filepath.Join(dir, "main.py"), "app = FastAPI()\n", 0o644)

	module, attr, err := locateApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	if module != "backend.api" || attr != "application" {
		t.Errorf("entrypoint = %s:%s", module, attr)
	}
}

func TestHeuristicFindsConventionalEntryFiles(t *testing.T) {
	tests := []struct {
		file       string
		content    string
		wantModule string
		wantAttr   string
	}{
		{"main.py", "app = FastAPI()\n", "main", "app"},
		{"app/main.py", "import fastapi\napi = fastapi.FastAPI(title='x')\n", "app.main", "api"},
		{"src/app.py", "application = FastAPI()\n", "src.app", "application"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tt.file), tt.content, 0o644)
			module, attr, err := locateApp(dir)
			if err != nil {
				t.Fatal(err)
			}
			if module != tt.wantModule || attr != tt.wantAttr {
				t.Errorf("got %s:%s, want %s:%s", module, attr, tt.wantModule, tt.wantAttr)
			}
		})
	}
}

func TestLocateAppFailsWithGuidance(t *testing.T) {
	_, _, err := locateApp(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "tool.payk") {
		t.Errorf("error should point at the entrypoint option, got: %v", err)
	}
}

func TestLocateInterpreterPrefersVenv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".venv", "bin", "python"), "#!/bin/sh\n", 0o755)

	interp, err := locateInterpreter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(interp) != 1 || interp[0] != filepath.Join(dir, ".venv", "bin", "python") {
		t.Errorf("interpreter = %v", interp)
	}
}

func TestCanHandle(t *testing.T) {
	imp := New()

	if imp.CanHandle(t.TempDir()) {
		t.Error("empty directory must be rejected")
	}
	if imp.CanHandle("not-a-dir") {
		t.Error("nonexistent path must be rejected")
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.py"), "app = FastAPI()\n", 0o644)
	if !imp.CanHandle(dir) {
		t.Error("directory with a FastAPI entry file must be handled")
	}

	pyDir := t.TempDir()
	writeFile(t, filepath.Join(pyDir, "pyproject.toml"),
		"[project]\ndependencies = [\"fastapi\"]\n", 0o644)
	if !imp.CanHandle(pyDir) {
		t.Error("directory whose pyproject mentions fastapi must be handled")
	}
}
