// Package core holds the domain types shared by storage, importers, and the
// TUI. It must stay free of Bubble Tea imports so it is testable headlessly.
package core

import "strings"

// KV is an ordered slice element, not a map entry, so YAML output stays
// ordered and diff-friendly.
type KV struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type Body struct {
	// Type is a payk body kind: json, text, or form.
	Type    string `yaml:"type,omitempty"`
	Content string `yaml:"content,omitempty"`
}

// IsZero reports whether the body is empty; yaml omitempty relies on it.
func (b Body) IsZero() bool {
	return b.Type == "" && b.Content == ""
}

const (
	AuthNone   = ""
	AuthBearer = "bearer"
	AuthBasic  = "basic"
)

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

type Request struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Method      string `yaml:"method"`
	URL         string `yaml:"url"`
	Params      []KV   `yaml:"params,omitempty"`
	Headers     []KV   `yaml:"headers,omitempty"`
	Body        Body   `yaml:"body,omitempty"`
	Auth        Auth   `yaml:"auth,omitempty"`
}

// Clone returns a deep copy, used to snapshot a request before sending so
// concurrent edits in the UI cannot race with the send goroutine.
func (r *Request) Clone() *Request {
	clone := *r
	clone.Params = append([]KV(nil), r.Params...)
	clone.Headers = append([]KV(nil), r.Headers...)
	return &clone
}

func (r *Request) Normalize(fallbackName string) {
	r.Method = strings.ToUpper(strings.TrimSpace(r.Method))
	if r.Method == "" {
		r.Method = "GET"
	}
	if strings.TrimSpace(r.Name) == "" {
		r.Name = fallbackName
	}
}
