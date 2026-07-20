// Package openapi imports OpenAPI 3.0/3.1 specifications as collections.
// Input can be a file path, an http(s) URL, or raw spec content. The tree is
// grouped by tags (falling back to the first path segment), request bodies
// get examples generated from schemas, and server URLs are carried over into
// environments as base_url.
package openapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/yusupkhemraev/payk/internal/core"
)

const fetchTimeout = 30 * time.Second

// Importer implements importer.Importer for OpenAPI specs. It is not safe
// for concurrent use: Warnings and Environments report the last Import.
type Importer struct {
	warnings     []string
	environments []core.Environment
}

// New returns an OpenAPI importer.
func New() *Importer {
	return &Importer{}
}

func (i *Importer) Name() string { return "openapi" }

// Warnings returns non-fatal issues from the last Import.
func (i *Importer) Warnings() []string { return i.warnings }

// Environments returns environments derived from server URLs during the
// last Import.
func (i *Importer) Environments() []core.Environment { return i.environments }

// CanHandle accepts spec file paths, http(s) URLs, and raw content with
// openapi/swagger markers.
func (i *Importer) CanHandle(input string) bool {
	trimmed := strings.TrimSpace(input)
	switch {
	case strings.HasPrefix(trimmed, "http://"), strings.HasPrefix(trimmed, "https://"):
		return true
	case looksLikeSpecContent(trimmed):
		return true
	default:
		return isSpecFile(trimmed)
	}
}

func looksLikeSpecContent(s string) bool {
	if len(s) > 200 {
		s = s[:200]
	}
	return strings.Contains(s, `"openapi"`) || strings.Contains(s, "openapi:") ||
		strings.Contains(s, `"swagger"`) || strings.Contains(s, "swagger:")
}

func isSpecFile(path string) bool {
	lower := strings.ToLower(path)
	if !strings.HasSuffix(lower, ".json") && !strings.HasSuffix(lower, ".yaml") &&
		!strings.HasSuffix(lower, ".yml") {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Import loads and parses the spec, producing a collection tree.
func (i *Importer) Import(ctx context.Context, input string) (*core.Collection, error) {
	i.warnings = nil
	i.environments = nil

	data, err := load(ctx, input)
	if err != nil {
		return nil, err
	}

	doc, err := libopenapi.NewDocument(data)
	if err != nil {
		return nil, fmt.Errorf("openapi: parse: %w", err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("openapi: build model (only 3.0/3.1 supported): %w", err)
	}

	b := &builder{importer: i}
	return b.build(&model.Model), nil
}

func load(ctx context.Context, input string) ([]byte, error) {
	trimmed := strings.TrimSpace(input)
	switch {
	case strings.HasPrefix(trimmed, "http://"), strings.HasPrefix(trimmed, "https://"):
		return fetch(ctx, trimmed)
	case isSpecFile(trimmed):
		data, err := os.ReadFile(trimmed)
		if err != nil {
			return nil, fmt.Errorf("openapi: read spec: %w", err)
		}
		return data, nil
	default:
		return []byte(input), nil
	}
}

func fetch(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("openapi: fetch: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openapi: fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openapi: fetch %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openapi: fetch: %w", err)
	}
	return data, nil
}

// builder assembles the collection while collecting warnings.
type builder struct {
	importer *Importer
	folders  []*core.Folder
	byName   map[string]*core.Folder
	security *securityIndex
}

func (b *builder) warn(format string, args ...any) {
	b.importer.warnings = append(b.importer.warnings, fmt.Sprintf(format, args...))
}

func (b *builder) build(doc *v3.Document) *core.Collection {
	name := "openapi"
	if doc.Info != nil && strings.TrimSpace(doc.Info.Title) != "" {
		name = slug(doc.Info.Title)
	}
	col := &core.Collection{Name: name}
	b.byName = map[string]*core.Folder{}
	b.security = newSecurityIndex(doc)

	b.importer.environments = serverEnvironments(name, doc.Servers)
	if len(b.importer.environments) == 0 {
		b.warn("spec defines no servers; set base_url in an environment before sending")
	}

	if doc.Paths != nil {
		for path, item := range doc.Paths.PathItems.FromOldest() {
			for method, op := range item.GetOperations().FromOldest() {
				req := b.buildRequest(strings.ToUpper(method), path, op)
				if folderName := groupFor(op, path); folderName == "" {
					col.Requests = append(col.Requests, req)
				} else {
					b.folderNamed(folderName).Requests = append(b.folderNamed(folderName).Requests, req)
				}
			}
		}
	}

	if vars := b.security.summary(); vars != nil {
		b.warn("spec declares authentication; define %s in an environment or process env",
			strings.Join(vars, ", "))
	}

	col.Folders = b.folders
	return col
}

func (b *builder) folderNamed(name string) *core.Folder {
	if f, ok := b.byName[name]; ok {
		return f
	}
	f := &core.Folder{Name: name}
	b.byName[name] = f
	b.folders = append(b.folders, f)
	return f
}

// groupFor picks the folder: the first tag, else the first path segment.
func groupFor(op *v3.Operation, path string) string {
	if len(op.Tags) > 0 && strings.TrimSpace(op.Tags[0]) != "" {
		return slug(op.Tags[0])
	}
	segment := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)[0]
	if segment == "" || strings.HasPrefix(segment, "{") {
		return ""
	}
	return slug(segment)
}

func (b *builder) buildRequest(method, path string, op *v3.Operation) *core.Request {
	req := &core.Request{
		Name:   requestName(method, path, op),
		Method: method,
		URL:    "{{base_url}}" + path,
	}

	for _, param := range op.Parameters {
		kv := core.KV{Name: param.Name, Value: paramExample(param)}
		switch param.In {
		case "query":
			req.Params = append(req.Params, kv)
		case "header":
			req.Headers = append(req.Headers, kv)
		}
	}

	b.security.apply(req, op)
	b.attachBody(req, op)
	return req
}

func requestName(method, path string, op *v3.Operation) string {
	if s := strings.TrimSpace(op.Summary); s != "" {
		return s
	}
	if op.OperationId != "" {
		return op.OperationId
	}
	return method + " " + path
}

func serverEnvironments(name string, servers []*v3.Server) []core.Environment {
	var envs []core.Environment
	for idx, server := range servers {
		url := strings.TrimSuffix(strings.TrimSpace(server.URL), "/")
		if url == "" {
			continue
		}
		envName := name
		if idx > 0 {
			envName = fmt.Sprintf("%s-%d", name, idx+1)
		}
		envs = append(envs, core.Environment{
			Name: envName,
			Vars: map[string]string{"base_url": url},
		})
	}
	return envs
}

// slug makes a name safe to use as a directory or environment name.
// Letters and digits of any script are kept, so tags like "Пользователи"
// group into their own folders instead of collapsing.
func slug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "openapi"
	}
	return b.String()
}
