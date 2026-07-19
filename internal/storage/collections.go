package storage

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/yusupkhemraev/payk/internal/core"
)

const yamlIndent = 2

// LoadCollections reads the whole collections tree. A missing collections
// directory yields an empty slice, not an error.
func (w *Workspace) LoadCollections() ([]*core.Collection, error) {
	entries, err := os.ReadDir(w.CollectionsDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: read collections dir: %w", err)
	}

	var collections []*core.Collection
	for _, entry := range sortedDirs(entries) {
		folder, err := loadFolder(filepath.Join(w.CollectionsDir(), entry.Name()))
		if err != nil {
			return nil, err
		}
		collections = append(collections, &core.Collection{
			Name:     entry.Name(),
			Folders:  folder.Folders,
			Requests: folder.Requests,
		})
	}
	return collections, nil
}

func loadFolder(dir string) (*core.Folder, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("storage: read folder: %w", err)
	}

	folder := &core.Folder{Name: filepath.Base(dir)}
	for _, entry := range sortedDirs(entries) {
		sub, err := loadFolder(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		folder.Folders = append(folder.Folders, sub)
	}
	for _, entry := range sortedRequestFiles(entries) {
		req, err := loadRequest(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		folder.Requests = append(folder.Requests, req)
	}
	return folder, nil
}

func loadRequest(path string) (*core.Request, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("storage: read request: %w", err)
	}
	var req core.Request
	if err := yaml.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("storage: parse %s: %w", path, err)
	}
	req.Normalize(requestNameFromFile(path))
	return &req, nil
}

// SaveRequest writes a request to <collections>/<collection>/<folders...>/,
// creating directories as needed. The file name derives from the request
// name; the marshaled key order is fixed by the struct, so repeated saves
// produce identical bytes and clean diffs.
func (w *Workspace) SaveRequest(collection string, folders []string, req *core.Request) error {
	dir := filepath.Join(w.CollectionsDir(), collection, filepath.Join(folders...))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("storage: create folder: %w", err)
	}

	data, err := marshalYAML(req)
	if err != nil {
		return fmt.Errorf("storage: marshal request: %w", err)
	}

	path := filepath.Join(dir, slug(req.Name)+".yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("storage: write request: %w", err)
	}
	return nil
}

// RenameRequest renames a request in place: the name field changes and the
// file moves to the slug of the new name.
func (w *Workspace) RenameRequest(collection string, folders []string, req *core.Request, newName string) error {
	dir := filepath.Join(w.CollectionsDir(), collection, filepath.Join(folders...))
	oldPath := filepath.Join(dir, slug(req.Name)+".yaml")

	req.Name = newName
	if err := w.SaveRequest(collection, folders, req); err != nil {
		return err
	}
	newPath := filepath.Join(dir, slug(newName)+".yaml")
	if oldPath != newPath {
		if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("storage: remove old request file: %w", err)
		}
	}
	return nil
}

// RenameFolder renames a folder directory. An empty path renames the
// collection itself.
func (w *Workspace) RenameFolder(collection string, folders []string, newName string) error {
	parent := filepath.Join(w.CollectionsDir(), collection, filepath.Join(folders...))
	newPath := filepath.Join(filepath.Dir(parent), slug(newName))
	if parent == newPath {
		return nil
	}
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("storage: %q already exists", filepath.Base(newPath))
	}
	if err := os.Rename(parent, newPath); err != nil {
		return fmt.Errorf("storage: rename folder: %w", err)
	}
	return nil
}

// DeleteRequest removes a request file.
func (w *Workspace) DeleteRequest(collection string, folders []string, req *core.Request) error {
	path := filepath.Join(w.CollectionsDir(), collection, filepath.Join(folders...), slug(req.Name)+".yaml")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("storage: delete request: %w", err)
	}
	return nil
}

// DeleteFolder removes a folder directory recursively. An empty path deletes
// the whole collection.
func (w *Workspace) DeleteFolder(collection string, folders []string) error {
	dir := filepath.Join(w.CollectionsDir(), collection, filepath.Join(folders...))
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("storage: delete folder: %w", err)
	}
	return nil
}

func marshalYAML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(yamlIndent)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func requestNameFromFile(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// slug converts a request name to a safe, stable file name.
func slug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "request"
	}
	return b.String()
}

func sortedDirs(entries []os.DirEntry) []os.DirEntry {
	var dirs []os.DirEntry
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, e)
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
	return dirs
}

func sortedRequestFiles(entries []os.DirEntry) []os.DirEntry {
	var files []os.DirEntry
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if !e.IsDir() && (ext == ".yaml" || ext == ".yml") {
			files = append(files, e)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	return files
}
