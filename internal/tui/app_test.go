package tui

import (
	"bytes"
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

func TestFocusSwitchingUpdatesStatusBar(t *testing.T) {
	tm := teatest.NewTestModel(t, New(testConfig(t)), teatest.WithInitialTermSize(140, 40))
	waitForOutput(t, tm, "collections")

	tm.Send(keyPress('l'))
	waitForOutput(t, tm, "request")

	tm.Send(keyPress('l'))
	waitForOutput(t, tm, "response")

	tm.Send(keyPress('q'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
