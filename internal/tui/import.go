package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/core"
	"github.com/yusupkhemraev/payk/internal/importer"
	"github.com/yusupkhemraev/payk/internal/storage"
)

// importFinishedMsg carries the outcome of an import: the reloaded tree plus
// non-fatal warnings from the importer. environments is non-nil when the
// import added or updated environments (e.g. OpenAPI server URLs).
type importFinishedMsg struct {
	workspace     *storage.Workspace
	collections   []*core.Collection
	environments  *core.Environments
	importSources []string
	imported      string
	warnings      []string
	err           error
}

// runImportCmd feeds the input through the first matching importer, saves
// the resulting requests into the workspace, and reloads the tree. When no
// workspace exists yet, a project-local .payk is created.
func runImportCmd(importers []importer.Importer, ws *storage.Workspace, input string) tea.Cmd {
	return func() tea.Msg {
		imp := importer.Find(importers, input)
		if imp == nil {
			return importFinishedMsg{workspace: ws, err: errors.New("no importer can handle this input")}
		}

		collection, err := imp.Import(context.Background(), input)
		if err != nil {
			return importFinishedMsg{workspace: ws, err: err}
		}

		if ws == nil {
			wd, err := os.Getwd()
			if err != nil {
				return importFinishedMsg{err: err}
			}
			ws = &storage.Workspace{Dir: filepath.Join(wd, ".payk")}
		}

		// Spec-based imports replace the collection so removed or renamed
		// routes disappear on re-import; curl imports keep appending to
		// theirs. The source is recorded to power :reimport.
		if imp.Name() != "curl" {
			if err := ws.DeleteFolder(collection.Name, nil); err != nil {
				return importFinishedMsg{workspace: ws, err: err}
			}
		}
		if err := saveCollection(ws, collection); err != nil {
			return importFinishedMsg{workspace: ws, err: err}
		}
		if imp.Name() != "curl" {
			if err := ws.SaveImportSource(collection.Name, input); err != nil {
				return importFinishedMsg{workspace: ws, err: err}
			}
		}

		environments, err := mergeEnvironments(ws, importer.EnvironmentsOf(imp))
		if err != nil {
			return importFinishedMsg{workspace: ws, err: err}
		}

		collections, err := ws.LoadCollections()
		msg := importFinishedMsg{
			workspace:    ws,
			collections:  collections,
			environments: environments,
			imported:     importedLabel(collection),
			warnings:     importer.WarningsOf(imp),
			err:          err,
		}
		if sources, err := ws.ImportSources(); err == nil {
			for name := range sources {
				msg.importSources = append(msg.importSources, name)
			}
			sort.Strings(msg.importSources)
		}
		return msg
	}
}

// mergeEnvironments upserts importer-provided environments (e.g. OpenAPI
// servers) into the workspace config. Existing environments with the same
// name only get their base_url updated, preserving user-defined vars.
func mergeEnvironments(ws *storage.Workspace, imported []core.Environment) (*core.Environments, error) {
	if len(imported) == 0 {
		return nil, nil
	}
	envs, err := ws.LoadEnvironments()
	if err != nil {
		return nil, err
	}
	for _, env := range imported {
		if existing := envs.Get(env.Name); existing != nil {
			if existing.Vars == nil {
				existing.Vars = map[string]string{}
			}
			for name, value := range env.Vars {
				existing.Vars[name] = value
			}
			continue
		}
		envs.Environments = append(envs.Environments, env)
	}
	if err := ws.SaveEnvironments(envs); err != nil {
		return nil, err
	}
	return envs, nil
}

// saveCollection persists every request of an imported collection, walking
// nested folders.
func saveCollection(ws *storage.Workspace, col *core.Collection) error {
	for _, req := range col.Requests {
		if err := ws.SaveRequest(col.Name, nil, req); err != nil {
			return err
		}
	}
	var walk func(path []string, folders []*core.Folder) error
	walk = func(path []string, folders []*core.Folder) error {
		for _, folder := range folders {
			folderPath := append(append([]string{}, path...), folder.Name)
			for _, req := range folder.Requests {
				if err := ws.SaveRequest(col.Name, folderPath, req); err != nil {
					return err
				}
			}
			if err := walk(folderPath, folder.Folders); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(nil, col.Folders)
}

func importedLabel(col *core.Collection) string {
	total := countRequests(col)
	if total == 1 && len(col.Requests) == 1 {
		return col.Requests[0].Name
	}
	return fmt.Sprintf("%d requests into %s", total, col.Name)
}

func countRequests(col *core.Collection) int {
	total := len(col.Requests)
	var walk func([]*core.Folder)
	walk = func(folders []*core.Folder) {
		for _, f := range folders {
			total += len(f.Requests)
			walk(f.Folders)
		}
	}
	walk(col.Folders)
	return total
}
