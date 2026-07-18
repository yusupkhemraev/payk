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
	workspace   *storage.Workspace
	collections []*core.Collection
	err         error
}

// loadWorkspaceCmd discovers the workspace and reads the collections tree.
// All file IO happens here, inside a command, never in Update.
func loadWorkspaceCmd(cfg Config) tea.Cmd {
	return func() tea.Msg {
		ws, err := resolveWorkspace(cfg)
		if errors.Is(err, storage.ErrNotFound) {
			return workspaceLoadedMsg{}
		}
		if err != nil {
			return workspaceLoadedMsg{err: err}
		}
		collections, err := ws.LoadCollections()
		return workspaceLoadedMsg{workspace: ws, collections: collections, err: err}
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
