// Package httpc sends requests built from core domain types and captures
// per-phase timings via httptrace. It has no TUI dependencies.
package httpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/yusupkhemraev/payk/internal/core"
)

// maxBodySize caps how much of a response body is kept in memory.
// A var, not a const, so tests can lower it.
var maxBodySize int64 = 20 << 20

// Timings holds per-phase durations captured with httptrace. Phases that did
// not happen (e.g. reused connection, plain HTTP) stay zero.
type Timings struct {
	DNS     time.Duration
	Connect time.Duration
	TLS     time.Duration
	// TTFB is measured from the start of the round trip to the first
	// response byte.
	TTFB  time.Duration
	Total time.Duration
}

// Response is a fully read HTTP response with timing data.
type Response struct {
	Status     string
	StatusCode int
	Proto      string
	Headers    http.Header
	Body       []byte
	// Truncated is set when the body exceeded the in-memory cap.
	Truncated bool
	Timings   Timings
}

func (r *Response) Size() int {
	return len(r.Body)
}

// tracer collects httptrace events. Callbacks may fire from multiple
// goroutines (happy eyeballs), so all access is mutex-guarded.
type tracer struct {
	mu           sync.Mutex
	dnsStart     time.Time
	connectStart time.Time
	tlsStart     time.Time
	timings      Timings
}

func (t *tracer) clientTrace(start time.Time) *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.dnsStart = time.Now()
		},
		DNSDone: func(httptrace.DNSDoneInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.timings.DNS = time.Since(t.dnsStart)
		},
		ConnectStart: func(_, _ string) {
			t.mu.Lock()
			defer t.mu.Unlock()
			if t.connectStart.IsZero() {
				t.connectStart = time.Now()
			}
		},
		ConnectDone: func(_, _ string, err error) {
			t.mu.Lock()
			defer t.mu.Unlock()
			if err == nil && !t.connectStart.IsZero() {
				t.timings.Connect = time.Since(t.connectStart)
			}
		},
		TLSHandshakeStart: func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.tlsStart = time.Now()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			t.mu.Lock()
			defer t.mu.Unlock()
			if err == nil && !t.tlsStart.IsZero() {
				t.timings.TLS = time.Since(t.tlsStart)
			}
		},
		GotFirstResponseByte: func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.timings.TTFB = time.Since(start)
		},
	}
}

// Send performs the request and reads the whole body. The context cancels
// the request at any phase, including body download.
func Send(ctx context.Context, req *core.Request) (*Response, error) {
	httpReq, err := buildRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	tr := &tracer{}
	httpReq = httpReq.WithContext(httptrace.WithClientTrace(httpReq.Context(), tr.clientTrace(start)))

	// A fresh transport per send keeps timings honest: pooled connections
	// would skip DNS/connect/TLS phases on every request after the first.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("httpc: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize+1))
	if err != nil {
		return nil, fmt.Errorf("httpc: read body: %w", err)
	}
	truncated := int64(len(body)) > maxBodySize
	if truncated {
		body = body[:maxBodySize]
	}

	tr.mu.Lock()
	timings := tr.timings
	tr.mu.Unlock()
	timings.Total = time.Since(start)

	return &Response{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Proto:      resp.Proto,
		Headers:    resp.Header,
		Body:       body,
		Truncated:  truncated,
		Timings:    timings,
	}, nil
}

func buildRequest(ctx context.Context, req *core.Request) (*http.Request, error) {
	u, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil {
		return nil, fmt.Errorf("httpc: parse url: %w", err)
	}
	if u.Scheme == "" {
		u.Scheme = "http"
	}

	query := u.Query()
	for _, p := range req.Params {
		if p.Name != "" {
			query.Add(p.Name, p.Value)
		}
	}
	u.RawQuery = query.Encode()

	var body io.Reader
	if !req.Body.IsZero() {
		body = strings.NewReader(req.Body.Content)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("httpc: build request: %w", err)
	}

	for _, h := range req.Headers {
		if h.Name != "" {
			httpReq.Header.Add(h.Name, h.Value)
		}
	}
	if !req.Body.IsZero() && httpReq.Header.Get("Content-Type") == "" {
		if ct := defaultContentType(req.Body.Type); ct != "" {
			httpReq.Header.Set("Content-Type", ct)
		}
	}

	switch req.Auth.Type {
	case core.AuthBearer:
		httpReq.Header.Set("Authorization", "Bearer "+req.Auth.Token)
	case core.AuthBasic:
		httpReq.SetBasicAuth(req.Auth.User, req.Auth.Pass)
	}
	return httpReq, nil
}

func defaultContentType(bodyType string) string {
	switch bodyType {
	case "json":
		return "application/json"
	case "form":
		return "application/x-www-form-urlencoded"
	case "text":
		return "text/plain"
	}
	return ""
}
