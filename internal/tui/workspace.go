package tui

import (
	"errors"
	"os"
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/core"
	"github.com/yusupkhemraev/payk/internal/storage"
)

type workspaceLoadedMsg struct {
	workspace    *storage.Workspace
	collections  []*core.Collection
	environments *core.Environments
	// importSources lists collections with a recorded import source, for
	// :reimport completion.
	importSources []string
	err           error
}

type requestSavedMsg struct {
	name string
	err  error
}

type environmentsSavedMsg struct {
	err error
}

// All file IO happens here, inside a command, never in Update.
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
		if sources, err := ws.ImportSources(); err == nil {
			for name := range sources {
				msg.importSources = append(msg.importSources, name)
			}
			sort.Strings(msg.importSources)
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
