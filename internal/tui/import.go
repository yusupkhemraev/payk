package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/core"
	"github.com/yusupkhemraev/payk/internal/importer"
	"github.com/yusupkhemraev/payk/internal/storage"
)

// importFinishedMsg carries the outcome of an import: the reloaded tree plus
// non-fatal warnings from the importer.
type importFinishedMsg struct {
	workspace   *storage.Workspace
	collections []*core.Collection
	imported    string
	warnings    []string
	err         error
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

		if err := saveCollection(ws, collection); err != nil {
			return importFinishedMsg{workspace: ws, err: err}
		}

		collections, err := ws.LoadCollections()
		return importFinishedMsg{
			workspace:   ws,
			collections: collections,
			imported:    importedLabel(collection),
			warnings:    importer.WarningsOf(imp),
			err:         err,
		}
	}
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
