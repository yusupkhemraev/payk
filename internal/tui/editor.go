package tui

import (
	"fmt"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/core"
	"github.com/yusupkhemraev/payk/internal/storage"
	"github.com/yusupkhemraev/payk/internal/tui/panels"
)

// editorFinishedMsg carries the body text edited in $EDITOR.
type editorFinishedMsg struct {
	content string
	err     error
}

// openEditorCmd suspends the TUI and opens the request body in $EDITOR via a
// temp file, reloading the content on exit.
func openEditorCmd(msg panels.EditBodyRequestedMsg) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	ext := ".txt"
	if msg.BodyType == "json" || msg.BodyType == "" {
		ext = ".json"
	}
	tmp, err := os.CreateTemp("", "payk-body-*"+ext)
	if err != nil {
		return func() tea.Msg { return editorFinishedMsg{err: err} }
	}
	path := tmp.Name()
	if _, err := tmp.WriteString(msg.Content); err != nil {
		_ = tmp.Close()
		return func() tea.Msg { return editorFinishedMsg{err: err} }
	}
	_ = tmp.Close()

	cmd := exec.Command(editor, path)
	return tea.ExecProcess(cmd, func(execErr error) tea.Msg {
		defer func() { _ = os.Remove(path) }()
		if execErr != nil {
			return editorFinishedMsg{err: fmt.Errorf("%s: %w", editor, execErr)}
		}
		edited, err := os.ReadFile(path)
		if err != nil {
			return editorFinishedMsg{err: err}
		}
		return editorFinishedMsg{content: string(edited)}
	})
}

// renameCmd performs the storage rename and reloads the workspace.
func renameCmd(ws *storage.Workspace, cfg Config, msg panels.RenameRequestedMsg) tea.Cmd {
	return func() tea.Msg {
		var err error
		if msg.Request != nil {
			err = ws.RenameRequest(msg.Collection, msg.Path, msg.Request, msg.NewName)
		} else {
			err = ws.RenameFolder(msg.Collection, msg.Path, msg.NewName)
		}
		if err != nil {
			return panels.StatusNote{Text: "rename failed: " + err.Error(), IsErr: true}
		}
		return loadWorkspaceCmd(cfg)()
	}
}

// requestCreatedMsg carries the reloaded workspace plus the location of the
// just-created request so it can be opened in the editor.
type requestCreatedMsg struct {
	loaded     workspaceLoadedMsg
	collection string
	path       []string
	name       string
}

// createRequestCmd writes a fresh GET request to disk and reloads the tree.
func createRequestCmd(ws *storage.Workspace, cfg Config, msg panels.CreateRequestedMsg) tea.Cmd {
	return func() tea.Msg {
		req := &core.Request{Name: msg.Name, Method: "GET"}
		if err := ws.SaveRequest(msg.Collection, msg.Path, req); err != nil {
			return panels.StatusNote{Text: "create failed: " + err.Error(), IsErr: true}
		}
		loaded, ok := loadWorkspaceCmd(cfg)().(workspaceLoadedMsg)
		if !ok {
			return panels.StatusNote{Text: "create failed: workspace reload", IsErr: true}
		}
		return requestCreatedMsg{
			loaded:     loaded,
			collection: msg.Collection,
			path:       msg.Path,
			name:       msg.Name,
		}
	}
}

// deleteCmd removes a request file or folder directory and reloads.
func deleteCmd(ws *storage.Workspace, cfg Config, msg panels.DeleteRequestedMsg) tea.Cmd {
	return func() tea.Msg {
		var err error
		if msg.Request != nil {
			err = ws.DeleteRequest(msg.Collection, msg.Path, msg.Request)
		} else {
			err = ws.DeleteFolder(msg.Collection, msg.Path)
		}
		if err != nil {
			return panels.StatusNote{Text: "delete failed: " + err.Error(), IsErr: true}
		}
		return loadWorkspaceCmd(cfg)()
	}
}
