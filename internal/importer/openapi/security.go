package openapi

import (
	"sort"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/yusupkhemraev/payk/internal/core"
)

// Placeholder variables used for imported auth. They stay unresolved until
// the user defines them in an environment or the process env, so a send
// fails loudly instead of silently going out unauthenticated.
const (
	tokenVar    = "{{api_token}}"
	userVar     = "{{api_user}}"
	passwordVar = "{{api_password}}"
	apiKeyVar   = "{{api_key}}"
)

type securityIndex struct {
	schemes map[string]*v3.SecurityScheme
	global  []*base.SecurityRequirement
	// used collects the placeholder variables applied to requests, for a
	// single summary warning.
	used map[string]bool
}

func newSecurityIndex(doc *v3.Document) *securityIndex {
	idx := &securityIndex{schemes: map[string]*v3.SecurityScheme{}, global: doc.Security, used: map[string]bool{}}
	if doc.Components != nil && doc.Components.SecuritySchemes != nil {
		for name, scheme := range doc.Components.SecuritySchemes.FromOldest() {
			idx.schemes[name] = scheme
		}
	}
	return idx
}

// Operation-level requirements override the global ones; an explicit empty
// list disables auth.
func (idx *securityIndex) apply(req *core.Request, op *v3.Operation) {
	requirements := idx.global
	if op.Security != nil {
		requirements = op.Security
	}

	for _, requirement := range requirements {
		if requirement == nil || requirement.Requirements == nil {
			continue
		}
		for name := range requirement.Requirements.FromOldest() {
			if scheme, ok := idx.schemes[name]; ok && idx.applyScheme(req, scheme) {
				return
			}
		}
	}
}

func (idx *securityIndex) applyScheme(req *core.Request, scheme *v3.SecurityScheme) bool {
	switch {
	case scheme.Type == "http" && strings.EqualFold(scheme.Scheme, "bearer"):
		req.Auth = core.Auth{Type: core.AuthBearer, Token: tokenVar}
		idx.used[tokenVar] = true
		return true

	case scheme.Type == "http" && strings.EqualFold(scheme.Scheme, "basic"):
		req.Auth = core.Auth{Type: core.AuthBasic, User: userVar, Pass: passwordVar}
		idx.used[userVar] = true
		idx.used[passwordVar] = true
		return true

	case scheme.Type == "apiKey" && scheme.In == "header":
		req.Headers = append(req.Headers, core.KV{Name: scheme.Name, Value: apiKeyVar})
		idx.used[apiKeyVar] = true
		return true

	case scheme.Type == "apiKey" && scheme.In == "query":
		req.Params = append(req.Params, core.KV{Name: scheme.Name, Value: apiKeyVar})
		idx.used[apiKeyVar] = true
		return true

	case scheme.Type == "oauth2" || scheme.Type == "openIdConnect":
		req.Auth = core.Auth{Type: core.AuthBearer, Token: tokenVar}
		idx.used[tokenVar] = true
		return true
	}
	return false
}

func (idx *securityIndex) summary() []string {
	if len(idx.used) == 0 {
		return nil
	}
	vars := make([]string, 0, len(idx.used))
	for name := range idx.used {
		vars = append(vars, strings.Trim(name, "{}"))
	}
	sort.Strings(vars)
	return vars
}
