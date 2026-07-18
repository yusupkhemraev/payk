// Package core holds the domain types shared by storage, importers, and the
// TUI. It must stay free of Bubble Tea imports so it is testable headlessly.
package core

import "strings"

// KV is an ordered name/value pair used for query params and headers.
// A slice of KV keeps YAML output ordered and diff-friendly, unlike a map.
type KV struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// Body is a request body with a content type hint.
type Body struct {
	// Type is a payk body kind: json, text, or form.
	Type    string `yaml:"type,omitempty"`
	Content string `yaml:"content,omitempty"`
}

// IsZero reports whether the body is empty; yaml omitempty relies on it.
func (b Body) IsZero() bool {
	return b.Type == "" && b.Content == ""
}

// Auth types supported in the MVP.
const (
	AuthNone   = ""
	AuthBearer = "bearer"
	AuthBasic  = "basic"
)

// Auth is the request authentication config.
type Auth struct {
	Type  string `yaml:"type,omitempty"`
	Token string `yaml:"token,omitempty"`
	User  string `yaml:"user,omitempty"`
	Pass  string `yaml:"pass,omitempty"`
}

// IsZero reports whether auth is unset; yaml omitempty relies on it.
func (a Auth) IsZero() bool {
	return a == Auth{}
}

// Request is a single HTTP request definition.
type Request struct {
	Name    string `yaml:"name"`
	Method  string `yaml:"method"`
	URL     string `yaml:"url"`
	Params  []KV   `yaml:"params,omitempty"`
	Headers []KV   `yaml:"headers,omitempty"`
	Body    Body   `yaml:"body,omitempty"`
	Auth    Auth   `yaml:"auth,omitempty"`
}

// Clone returns a deep copy, used to snapshot a request before sending so
// concurrent edits in the UI cannot race with the send goroutine.
func (r *Request) Clone() *Request {
	clone := *r
	clone.Params = append([]KV(nil), r.Params...)
	clone.Headers = append([]KV(nil), r.Headers...)
	return &clone
}

// Normalize fills defaults: uppercase method, GET when method is empty, and
// name as fallback for callers that loaded a request without one.
func (r *Request) Normalize(fallbackName string) {
	r.Method = strings.ToUpper(strings.TrimSpace(r.Method))
	if r.Method == "" {
		r.Method = "GET"
	}
	if strings.TrimSpace(r.Name) == "" {
		r.Name = fallbackName
	}
}
