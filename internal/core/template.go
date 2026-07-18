package core

import (
	"regexp"
	"sort"
	"strings"
)

var placeholderRe = regexp.MustCompile(`\{\{\s*([^{}\s][^{}]*?)\s*\}\}`)

// envPrefix marks placeholders resolved from process environment variables
// instead of the active payk environment: {{env:API_TOKEN}}.
const envPrefix = "env:"

// resolver substitutes placeholders and records the ones it cannot resolve.
type resolver struct {
	vars      map[string]string
	lookupEnv func(string) (string, bool)
	missing   map[string]bool
}

// ResolveRequest returns a deep copy of req with every {{var}} and
// {{env:NAME}} placeholder substituted in URL, params, headers, body, and
// auth fields. Unresolvable placeholders are left intact and returned sorted
// so callers can refuse to send. The original request is never mutated —
// resolved values (secrets included) exist only in the returned copy.
func ResolveRequest(req *Request, vars map[string]string, lookupEnv func(string) (string, bool)) (*Request, []string) {
	r := &resolver{vars: vars, lookupEnv: lookupEnv, missing: map[string]bool{}}

	out := req.Clone()
	out.URL = r.resolve(out.URL)
	for i := range out.Params {
		out.Params[i].Name = r.resolve(out.Params[i].Name)
		out.Params[i].Value = r.resolve(out.Params[i].Value)
	}
	for i := range out.Headers {
		out.Headers[i].Name = r.resolve(out.Headers[i].Name)
		out.Headers[i].Value = r.resolve(out.Headers[i].Value)
	}
	out.Body.Content = r.resolve(out.Body.Content)
	out.Auth.Token = r.resolve(out.Auth.Token)
	out.Auth.User = r.resolve(out.Auth.User)
	out.Auth.Pass = r.resolve(out.Auth.Pass)

	return out, r.missingList()
}

func (r *resolver) resolve(s string) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	return placeholderRe.ReplaceAllStringFunc(s, func(match string) string {
		name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(match, "{{"), "}}"))

		if envName, ok := strings.CutPrefix(name, envPrefix); ok {
			if value, found := r.lookupEnv(strings.TrimSpace(envName)); found {
				return value
			}
			r.missing[name] = true
			return match
		}

		if value, found := r.vars[name]; found {
			return value
		}
		r.missing[name] = true
		return match
	})
}

func (r *resolver) missingList() []string {
	if len(r.missing) == 0 {
		return nil
	}
	names := make([]string, 0, len(r.missing))
	for name := range r.missing {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
