package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yusupkhemraev/payk/internal/httpc"
)

// respondWith builds a model that already holds the given response.
func respondWith(t *testing.T, handler http.HandlerFunc) tea.Model {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	var m tea.Model = New(Config{WorkspaceDir: serverWorkspace(t, srv.URL)})
	m = stepMsg(m, tea.WindowSizeMsg{Width: 120, Height: 24})
	m = runCmds(m, m.Init())
	m = stepMsg(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	model := m.(Model)
	resp, err := httpc.Send(context.Background(), model.request.CurrentRequest())
	if err != nil {
		t.Fatal(err)
	}
	m = stepMsg(m, responseReceivedMsg{resp: resp})
	// Focus the response pane.
	m = typeString(m, "ll")
	return m
}

func TestHeadersTabScrolls(t *testing.T) {
	m := respondWith(t, func(w http.ResponseWriter, r *http.Request) {
		for i := range 40 {
			w.Header().Set(fmt.Sprintf("X-Header-%02d", i), fmt.Sprintf("value-%02d", i))
		}
		_, _ = w.Write([]byte(`{}`))
	})

	m = typeString(m, "]") // headers tab
	view := plainView(m)
	if !strings.Contains(view, "X-Header-00 ") {
		t.Fatalf("first header missing:\n%s", view)
	}
	if strings.Contains(view, "X-Header-39 ") {
		t.Fatal("last header should be below the fold before scrolling")
	}
	if !strings.Contains(view, "1–") || !strings.Contains(view, "/43") {
		t.Errorf("scroll indicator missing:\n%s", view)
	}

	// G jumps to the bottom: the last header becomes visible.
	m = typeString(m, "G")
	view = plainView(m)
	if !strings.Contains(view, "X-Header-39 ") {
		t.Errorf("G should reveal the last header:\n%s", view)
	}
	// The percentage may be clipped by the frame; the range is enough.
	if !strings.Contains(view, "43/43") {
		t.Errorf("indicator should show the bottom range:\n%s", view)
	}
}

// collapseView strips layout noise so text split across wrapped lines can be
// matched as one string.
func collapseView(view string) string {
	return strings.NewReplacer("\n", "", "│", "", " ", "").Replace(view)
}

func TestBodyWrapToggle(t *testing.T) {
	// A long line with a unique tail marker far past the pane width.
	long := strings.Repeat("x", 280) + "TAIL-MARKER-END"
	m := respondWith(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(long))
	})

	// Without wrap the viewport clips the line: its tail is not visible.
	if strings.Contains(collapseView(plainView(m)), "TAIL-MARKER-END") {
		t.Fatalf("long line tail should be clipped without wrap:\n%s", plainView(m))
	}

	m = typeString(m, "w")
	view := plainView(m)
	if !strings.Contains(view, "wrap") {
		t.Errorf("wrap state missing from indicator:\n%s", view)
	}
	// The tail of the long line is now visible on the wrapped lines (the
	// marker may be split across a line break).
	if !strings.Contains(collapseView(view), "TAIL-MARKER-END") {
		t.Errorf("wrapped tail not visible:\n%s", view)
	}

	m = typeString(m, "w")
	if strings.Contains(collapseView(plainView(m)), "TAIL-MARKER-END") {
		t.Error("second w should disable wrapping")
	}
}

func TestSearchJumpWithWrapEnabled(t *testing.T) {
	var body strings.Builder
	body.WriteString(`{"filler": "` + strings.Repeat("x", 200) + `",`)
	for i := range 30 {
		fmt.Fprintf(&body, `"key%02d": "row",`, i)
	}
	body.WriteString(`"needle": "found-me"}`)

	m := respondWith(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body.String()))
	})

	m = typeString(m, "w") // wrap on
	m = typeString(m, "/")
	m = typeString(m, "needle")
	m = stepMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	view := plainView(m)
	if !strings.Contains(view, "/needle  1/1") {
		t.Fatalf("search should find the needle with wrap on:\n%s", view)
	}
	if !strings.Contains(view, "needle") {
		t.Errorf("match line should be scrolled into view:\n%s", view)
	}
}
