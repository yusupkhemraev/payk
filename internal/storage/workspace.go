// Package storage persists collections and environments as plain YAML files.
//
// Layout: a workspace is a ".payk" directory (project-local, discovered by
// walking up from the working directory) or "~/.config/payk" (global
// fallback). Inside it:
//
//	collections/<collection>/<folder...>/<request>.yaml
//	environments.yaml
//
// The tree of collections mirrors the filesystem: directories are folders,
// one YAML file per request. One file per request was chosen over one file
// per folder because it keeps diffs minimal — editing a request touches a
// single small file, and renames show up as file moves in git.
package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotFound is returned by Discover when no workspace exists.
var ErrNotFound = errors.New("no payk workspace found")

// Workspace is a payk data directory.
type Workspace struct {
	// Dir is the workspace root, i.e. the .payk directory itself.
	Dir string
}

// CollectionsDir returns the root directory of the collections tree.
func (w *Workspace) CollectionsDir() string {
	return filepath.Join(w.Dir, "collections")
}

// EnvironmentsFile returns the path of the environments config.
func (w *Workspace) EnvironmentsFile() string {
	return filepath.Join(w.Dir, "environments.yaml")
}

// Discover finds the nearest workspace: it walks up from start looking for a
// .payk directory, then falls back to the global workspace. Returns
// ErrNotFound when neither exists.
func Discover(start string) (*Workspace, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return nil, fmt.Errorf("storage: resolve start dir: %w", err)
	}

	for {
		candidate := filepath.Join(dir, ".payk")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return &Workspace{Dir: candidate}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	global, err := globalDir()
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(global); err == nil && info.IsDir() {
		return &Workspace{Dir: global}, nil
	}
	return nil, ErrNotFound
}

// globalDir returns ~/.config/payk without requiring it to exist.
func globalDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("storage: resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "payk"), nil
}
