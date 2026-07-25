package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// statusCacheFile records the last response per request so the tree can show
// it. It lives in a cache directory that is git-ignored: request YAML must
// stay untouched by sending, or every run would dirty the working tree.
const statusCacheFile = "status.json"

const cacheDirName = ".cache"

// StatusCache maps "collection/folder/.../request" to a short status label
// such as "200" or "ERR".
type StatusCache map[string]string

// StatusKey builds the cache key for a request location.
func StatusKey(collection string, folders []string, name string) string {
	parts := append([]string{collection}, folders...)
	parts = append(parts, name)
	return strings.Join(parts, "/")
}

func (w *Workspace) cacheDir() string {
	return filepath.Join(w.Dir, cacheDirName)
}

// LoadStatuses reads the cache; a missing file yields an empty map.
func (w *Workspace) LoadStatuses() (StatusCache, error) {
	data, err := os.ReadFile(filepath.Join(w.cacheDir(), statusCacheFile))
	if os.IsNotExist(err) {
		return StatusCache{}, nil
	}
	if err != nil {
		return StatusCache{}, fmt.Errorf("storage: read status cache: %w", err)
	}
	cache := StatusCache{}
	if err := json.Unmarshal(data, &cache); err != nil {
		return StatusCache{}, fmt.Errorf("storage: parse status cache: %w", err)
	}
	return cache, nil
}

// SaveStatuses writes the cache, creating the directory and its .gitignore.
func (w *Workspace) SaveStatuses(cache StatusCache) error {
	dir := w.cacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("storage: create cache dir: %w", err)
	}
	// The cache is machine-local noise; keep it out of commits.
	ignore := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(ignore); os.IsNotExist(err) {
		if err := os.WriteFile(ignore, []byte("*\n"), 0o644); err != nil {
			return fmt.Errorf("storage: write cache gitignore: %w", err)
		}
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("storage: marshal status cache: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, statusCacheFile), data, 0o644); err != nil {
		return fmt.Errorf("storage: write status cache: %w", err)
	}
	return nil
}
