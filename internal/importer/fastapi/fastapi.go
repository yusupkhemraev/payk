// Package fastapi imports a FastAPI project directory by asking the
// project's own Python interpreter for app.openapi() — no Python AST
// parsing. The resulting JSON feeds the OpenAPI importer.
package fastapi

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yusupkhemraev/payk/internal/core"
	"github.com/yusupkhemraev/payk/internal/importer/openapi"
)

const scanTimeout = 30 * time.Second

// stderrTail caps how much captured stderr is surfaced in errors.
const stderrTail = 2000

// Importer implements importer.Importer for FastAPI project directories.
// It is not safe for concurrent use: Warnings and Environments report the
// last Import.
type Importer struct {
	openapi *openapi.Importer
}

func New() *Importer {
	return &Importer{openapi: openapi.New()}
}

func (i *Importer) Name() string { return "fastapi" }

// Warnings forwards the underlying OpenAPI importer's warnings.
func (i *Importer) Warnings() []string { return i.openapi.Warnings() }

// Environments forwards environments derived from the generated spec.
func (i *Importer) Environments() []core.Environment { return i.openapi.Environments() }

// CanHandle accepts directories that look like FastAPI projects.
func (i *Importer) CanHandle(input string) bool {
	dir := strings.TrimSpace(input)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	if _, _, ok := entrypointFromPyproject(dir); ok {
		return true
	}
	if data, err := os.ReadFile(filepath.Join(dir, "pyproject.toml")); err == nil &&
		bytes.Contains(data, []byte("fastapi")) {
		return true
	}
	_, _, err = entrypointFromHeuristic(dir)
	return err == nil
}

// Import locates the interpreter and the app, extracts the OpenAPI spec via
// a subprocess, and feeds it to the OpenAPI importer.
func (i *Importer) Import(ctx context.Context, input string) (*core.Collection, error) {
	root, err := filepath.Abs(strings.TrimSpace(input))
	if err != nil {
		return nil, fmt.Errorf("fastapi: resolve project dir: %w", err)
	}

	module, attr, err := locateApp(root)
	if err != nil {
		return nil, err
	}
	interpreter, err := locateInterpreter(root)
	if err != nil {
		return nil, err
	}

	spec, err := extractSpec(ctx, root, interpreter, module, attr)
	if err != nil {
		return nil, err
	}

	col, err := i.openapi.Import(ctx, string(spec))
	if err != nil {
		return nil, err
	}
	// FastAPI's default title is "FastAPI"; the project directory name is a
	// far better collection name then.
	if col.Name == "fastapi" {
		col.Name = filepath.Base(root)
	}
	return col, nil
}

// locateApp finds "module:attr": the payk.entrypoint option in
// pyproject.toml wins, then a heuristic scan of conventional entry files.
func locateApp(root string) (module, attr string, err error) {
	if module, attr, ok := entrypointFromPyproject(root); ok {
		return module, attr, nil
	}
	return entrypointFromHeuristic(root)
}

var (
	sectionRe    = regexp.MustCompile(`^\s*\[(.+)]\s*$`)
	entrypointRe = regexp.MustCompile(`^\s*entrypoint\s*=\s*"([^"]+)"`)
	fastapiVarRe = regexp.MustCompile(`(?m)^\s*(\w+)\s*=\s*(?:fastapi\.)?FastAPI\s*\(`)
)

// entrypointFromPyproject reads the [tool.payk] entrypoint option, e.g.
// entrypoint = "app.main:app". A single-key section scan avoids pulling in a
// TOML parser.
func entrypointFromPyproject(root string) (string, string, bool) {
	data, err := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	if err != nil {
		return "", "", false
	}

	inPayk := false
	for line := range strings.SplitSeq(string(data), "\n") {
		if m := sectionRe.FindStringSubmatch(line); m != nil {
			inPayk = m[1] == "tool.payk"
			continue
		}
		if !inPayk {
			continue
		}
		if m := entrypointRe.FindStringSubmatch(line); m != nil {
			if module, attr, found := strings.Cut(m[1], ":"); found {
				return module, attr, true
			}
		}
	}
	return "", "", false
}

// entrypointFromHeuristic scans conventional entry files for a FastAPI()
// assignment and derives the module path from the file location.
func entrypointFromHeuristic(root string) (string, string, error) {
	candidates := []string{
		"main.py", "app.py",
		filepath.Join("app", "main.py"),
		filepath.Join("src", "main.py"),
		filepath.Join("src", "app.py"),
	}
	for _, rel := range candidates {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		if m := fastapiVarRe.FindSubmatch(data); m != nil {
			module := strings.TrimSuffix(rel, ".py")
			module = strings.ReplaceAll(module, string(filepath.Separator), ".")
			return module, string(m[1]), nil
		}
	}
	return "", "", fmt.Errorf(
		"fastapi: no FastAPI app found — set [tool.payk] entrypoint = \"module:attr\" in pyproject.toml")
}

// locateInterpreter finds the project's Python: .venv, then poetry, then uv.
// The result is an argv prefix, since uv runs python through its own CLI.
func locateInterpreter(root string) ([]string, error) {
	for _, candidate := range []string{
		filepath.Join(root, ".venv", "bin", "python"),
		filepath.Join(root, ".venv", "Scripts", "python.exe"),
	} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return []string{candidate}, nil
		}
	}

	if path, err := exec.LookPath("poetry"); err == nil {
		cmd := exec.Command(path, "env", "info", "-e")
		cmd.Dir = root
		if out, err := cmd.Output(); err == nil {
			if interp := strings.TrimSpace(string(out)); interp != "" {
				return []string{interp}, nil
			}
		}
	}

	if path, err := exec.LookPath("uv"); err == nil {
		return []string{path, "run", "python"}, nil
	}

	return nil, fmt.Errorf(
		"fastapi: no interpreter found (looked for .venv/bin/python, poetry, uv)")
}

// extractSpec runs the interpreter to import the app and print its OpenAPI
// schema as JSON. Import errors surface with the captured stderr.
func extractSpec(ctx context.Context, root string, interpreter []string, module, attr string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, scanTimeout)
	defer cancel()

	script := fmt.Sprintf(
		"import json, importlib; m = importlib.import_module('%s'); print(json.dumps(getattr(m, '%s').openapi()))",
		module, attr)
	args := append(append([]string{}, interpreter[1:]...), "-c", script)

	cmd := exec.CommandContext(ctx, interpreter[0], args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("fastapi: importing %s:%s timed out after %s", module, attr, scanTimeout)
		}
		return nil, fmt.Errorf("fastapi: importing %s:%s failed: %w\n%s",
			module, attr, err, tail(stderr.String(), stderrTail))
	}
	return stdout.Bytes(), nil
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
