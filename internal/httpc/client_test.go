package httpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusupkhemraev/payk/internal/core"
)

type echo struct {
	Method string      `json:"method"`
	Path   string      `json:"path"`
	Query  string      `json:"query"`
	Header http.Header `json:"header"`
	Body   string      `json:"body"`
}

func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(echo{
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.RawQuery,
			Header: r.Header,
			Body:   string(body),
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func sendEcho(t *testing.T, req *core.Request) (*Response, echo) {
	t.Helper()
	resp, err := Send(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	var e echo
	if err := json.Unmarshal(resp.Body, &e); err != nil {
		t.Fatalf("echo response not JSON: %v\n%s", err, resp.Body)
	}
	return resp, e
}

func TestSendBuildsFullRequest(t *testing.T) {
	srv := echoServer(t)
	resp, e := sendEcho(t, &core.Request{
		Method: "POST",
		URL:    srv.URL + "/users?existing=1",
		Params: []core.KV{{Name: "page", Value: "2"}},
		Headers: []core.KV{
			{Name: "X-Custom", Value: "abc"},
		},
		Body: core.Body{Type: "json", Content: `{"a":1}`},
		Auth: core.Auth{Type: core.AuthBearer, Token: "tok123"},
	})

	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	if e.Method != "POST" || e.Path != "/users" {
		t.Errorf("got %s %s", e.Method, e.Path)
	}
	if !strings.Contains(e.Query, "existing=1") || !strings.Contains(e.Query, "page=2") {
		t.Errorf("params not merged into query: %q", e.Query)
	}
	if e.Header.Get("X-Custom") != "abc" {
		t.Errorf("custom header lost: %v", e.Header)
	}
	if e.Header.Get("Authorization") != "Bearer tok123" {
		t.Errorf("bearer auth not applied: %q", e.Header.Get("Authorization"))
	}
	if e.Header.Get("Content-Type") != "application/json" {
		t.Errorf("json body should default content type, got %q", e.Header.Get("Content-Type"))
	}
	if e.Body != `{"a":1}` {
		t.Errorf("body = %q", e.Body)
	}
}

func TestSendBasicAuthAndExplicitContentType(t *testing.T) {
	srv := echoServer(t)
	_, e := sendEcho(t, &core.Request{
		Method:  "POST",
		URL:     srv.URL,
		Headers: []core.KV{{Name: "Content-Type", Value: "text/csv"}},
		Body:    core.Body{Type: "json", Content: "a,b"},
		Auth:    core.Auth{Type: core.AuthBasic, User: "u", Pass: "p"},
	})

	if !strings.HasPrefix(e.Header.Get("Authorization"), "Basic ") {
		t.Errorf("basic auth not applied: %q", e.Header.Get("Authorization"))
	}
	if e.Header.Get("Content-Type") != "text/csv" {
		t.Errorf("explicit content type must win, got %q", e.Header.Get("Content-Type"))
	}
}

func TestSendCapturesTimings(t *testing.T) {
	srv := echoServer(t)
	resp, _ := sendEcho(t, &core.Request{Method: "GET", URL: srv.URL})

	if resp.Timings.TTFB <= 0 {
		t.Errorf("TTFB = %v, want > 0", resp.Timings.TTFB)
	}
	if resp.Timings.Total < resp.Timings.TTFB {
		t.Errorf("Total %v < TTFB %v", resp.Timings.Total, resp.Timings.TTFB)
	}
	if resp.Timings.Connect <= 0 {
		t.Errorf("Connect = %v, want > 0 on a fresh connection", resp.Timings.Connect)
	}
}

func TestSendCancellation(t *testing.T) {
	blocked := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocked
	}))
	t.Cleanup(func() {
		close(blocked)
		srv.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := Send(ctx, &core.Request{Method: "GET", URL: srv.URL})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Error("cancellation took too long")
	}
}

func TestSendTruncatesHugeBody(t *testing.T) {
	old := maxBodySize
	maxBodySize = 1024
	t.Cleanup(func() { maxBodySize = old })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, 5000))
	}))
	t.Cleanup(srv.Close)

	resp, err := Send(context.Background(), &core.Request{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Truncated {
		t.Error("want Truncated = true")
	}
	if resp.Size() != 1024 {
		t.Errorf("kept %d bytes, want 1024", resp.Size())
	}
}

func TestSendBadURL(t *testing.T) {
	_, err := Send(context.Background(), &core.Request{Method: "GET", URL: "http://\x00bad"})
	if err == nil {
		t.Fatal("want error for invalid URL")
	}
}
