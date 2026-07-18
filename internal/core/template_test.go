package core

import (
	"reflect"
	"testing"
)

func noEnv(string) (string, bool) { return "", false }

func testEnv(vars map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := vars[name]
		return v, ok
	}
}

func TestResolveRequestSubstitutesAllFields(t *testing.T) {
	req := &Request{
		Name:   "login",
		Method: "POST",
		URL:    "{{base_url}}/auth?x={{flag}}",
		Params: []KV{{Name: "{{pname}}", Value: "{{pval}}"}},
		Headers: []KV{
			{Name: "X-Env", Value: "{{ spaced }}"},
		},
		Body: Body{Type: "json", Content: `{"token": "{{env:API_TOKEN}}"}`},
		Auth: Auth{Type: AuthBearer, Token: "{{env:API_TOKEN}}"},
	}
	vars := map[string]string{
		"base_url": "https://api.example.com",
		"flag":     "1",
		"pname":    "page",
		"pval":     "2",
		"spaced":   "ok",
	}

	out, missing := ResolveRequest(req, vars, testEnv(map[string]string{"API_TOKEN": "s3cr3t"}))

	if missing != nil {
		t.Fatalf("unexpected missing vars: %v", missing)
	}
	if out.URL != "https://api.example.com/auth?x=1" {
		t.Errorf("url = %q", out.URL)
	}
	if out.Params[0].Name != "page" || out.Params[0].Value != "2" {
		t.Errorf("params = %+v", out.Params)
	}
	if out.Headers[0].Value != "ok" {
		t.Errorf("spaced placeholder not resolved: %q", out.Headers[0].Value)
	}
	if out.Body.Content != `{"token": "s3cr3t"}` {
		t.Errorf("body = %q", out.Body.Content)
	}
	if out.Auth.Token != "s3cr3t" {
		t.Errorf("auth token = %q", out.Auth.Token)
	}
}

func TestResolveRequestNeverMutatesOriginal(t *testing.T) {
	req := &Request{
		URL:     "{{base_url}}/x",
		Headers: []KV{{Name: "A", Value: "{{v}}"}},
		Auth:    Auth{Type: AuthBearer, Token: "{{env:T}}"},
	}
	_, _ = ResolveRequest(req, map[string]string{"base_url": "b", "v": "1"},
		testEnv(map[string]string{"T": "secret"}))

	if req.URL != "{{base_url}}/x" || req.Headers[0].Value != "{{v}}" || req.Auth.Token != "{{env:T}}" {
		t.Errorf("original request mutated: %+v", req)
	}
}

func TestResolveRequestCollectsMissingSorted(t *testing.T) {
	req := &Request{
		URL:  "{{zeta}}/{{alpha}}/{{zeta}}",
		Body: Body{Content: "{{env:GONE}}"},
	}
	out, missing := ResolveRequest(req, nil, noEnv)

	want := []string{"alpha", "env:GONE", "zeta"}
	if !reflect.DeepEqual(missing, want) {
		t.Errorf("missing = %v, want %v", missing, want)
	}
	if out.URL != "{{zeta}}/{{alpha}}/{{zeta}}" {
		t.Errorf("unresolved placeholders must stay intact: %q", out.URL)
	}
}

func TestResolveRequestEmptyEnvValueIsFound(t *testing.T) {
	req := &Request{URL: "{{env:EMPTY}}/x"}
	out, missing := ResolveRequest(req, nil, testEnv(map[string]string{"EMPTY": ""}))

	if missing != nil {
		t.Errorf("set-but-empty env var must not be missing: %v", missing)
	}
	if out.URL != "/x" {
		t.Errorf("url = %q", out.URL)
	}
}

func TestResolveRequestPlainStringsUntouched(t *testing.T) {
	req := &Request{URL: "https://example.com/{id}", Body: Body{Content: "{not a var}"}}
	out, missing := ResolveRequest(req, nil, noEnv)

	if missing != nil {
		t.Errorf("unexpected missing: %v", missing)
	}
	if out.URL != req.URL || out.Body.Content != req.Body.Content {
		t.Errorf("plain strings changed: %+v", out)
	}
}
