package tui

import (
	"errors"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/core"
	"github.com/yusupkhemraev/payk/internal/storage"
)

// workspaceLoadedMsg carries the result of the initial workspace load.
type workspaceLoadedMsg struct {
	workspace    *storage.Workspace
	collections  []*core.Collection
	environments *core.Environments
	err          error
}

// requestSavedMsg reports the outcome of a :w save.
type requestSavedMsg struct {
	name string
	err  error
}

// environmentsSavedMsg reports persisting the active environment switch.
type environmentsSavedMsg struct {
	err error
}

// loadWorkspaceCmd discovers the workspace and reads the collections tree
// and environments. All file IO happens here, inside a command, never in
// Update.
func loadWorkspaceCmd(cfg Config) tea.Cmd {
	return func() tea.Msg {
		ws, err := resolveWorkspace(cfg)
		if errors.Is(err, storage.ErrNotFound) {
			return workspaceLoadedMsg{environments: &core.Environments{}}
		}
		if err != nil {
			return workspaceLoadedMsg{err: err, environments: &core.Environments{}}
		}
		collections, err := ws.LoadCollections()
		msg := workspaceLoadedMsg{workspace: ws, collections: collections, err: err}
		msg.environments, _ = ws.LoadEnvironments()
		if msg.environments == nil {
			msg.environments = &core.Environments{}
		}
		return msg
	}
}

// saveRequestCmd persists a snapshot of the request as it is in the editor —
// placeholders intact, never resolved values.
func saveRequestCmd(ws *storage.Workspace, collection string, path []string, req *core.Request) tea.Cmd {
	snapshot := req.Clone()
	return func() tea.Msg {
		return requestSavedMsg{name: snapshot.Name, err: ws.SaveRequest(collection, path, snapshot)}
	}
}

// saveEnvironmentsCmd persists the environments config (e.g. the active
// environment after :env).
func saveEnvironmentsCmd(ws *storage.Workspace, envs *core.Environments) tea.Cmd {
	snapshot := *envs
	return func() tea.Msg {
		return environmentsSavedMsg{err: ws.SaveEnvironments(&snapshot)}
	}
}

func resolveWorkspace(cfg Config) (*storage.Workspace, error) {
	if cfg.WorkspaceDir != "" {
		return &storage.Workspace{Dir: cfg.WorkspaceDir}, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return storage.Discover(wd)
}
