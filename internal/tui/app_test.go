package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"
)

func keyPress(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: string(code)}
}

// testConfig points the app at an empty temp workspace so tests never pick
// up a real one from the environment.
func testConfig(t *testing.T) Config {
	t.Helper()
	return Config{WorkspaceDir: t.TempDir()}
}

// waitForOutput blocks until every marker has appeared in program output.
// WaitFor drains the output stream, so markers rendered together must be
// awaited in a single call.
func waitForOutput(t *testing.T, tm *teatest.TestModel, markers ...string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		for _, marker := range markers {
			if !bytes.Contains(bts, []byte(marker)) {
				return false
			}
		}
		return true
	}, teatest.WithDuration(3*time.Second))
}

func TestStartupShowsAllPanesAndQuits(t *testing.T) {
	tm := teatest.NewTestModel(t, New(testConfig(t)), teatest.WithInitialTermSize(140, 40))

	waitForOutput(t, tm, "Collections", "Request", "Response", "payk")

	tm.Send(keyPress('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestResizeToTinyTerminalKeepsRendering(t *testing.T) {
	tm := teatest.NewTestModel(t, New(testConfig(t)), teatest.WithInitialTermSize(140, 40))
	waitForOutput(t, tm, "Collections")

	tm.Send(tea.WindowSizeMsg{Width: 60, Height: 20})
	waitForOutput(t, tm, "payk")

	tm.Send(tea.WindowSizeMsg{Width: 100, Height: 30})
	waitForOutput(t, tm, "payk")

	tm.Send(keyPress('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestHelpOverlayTogglesOpenAndClosed(t *testing.T) {
	tm := teatest.NewTestModel(t, New(testConfig(t)), teatest.WithInitialTermSize(140, 40))
	waitForOutput(t, tm, "Collections")

	tm.Send(keyPress('?'))
	waitForOutput(t, tm, "keybindings")

	tm.Send(keyPress('?'))
	waitForOutput(t, tm, "Collections")

	tm.Send(keyPress('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// Checked synchronously: the terminal diff only emits changed characters, so
// consecutive focus names ("request" → "response") never appear in full in
// the raw output stream.
func TestFocusSwitchingUpdatesStatusBar(t *testing.T) {
	var m tea.Model = New(testConfig(t))
	m = stepMsg(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m = runCmds(m, m.Init())

	for _, want := range []string{"collections", "request", "response"} {
		if !strings.Contains(plainView(m), " "+want+" ") {
			t.Errorf("status bar should show focus %q:\n%s", want, plainView(m))
		}
		m = typeString(m, "l")
	}
}
