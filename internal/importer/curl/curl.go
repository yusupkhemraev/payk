// Package curl imports a curl command line as a single-request collection.
package curl

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	shellwords "github.com/mattn/go-shellwords"

	"github.com/yusupkhemraev/payk/internal/core"
)

// CollectionName is the collection curl imports land in.
const CollectionName = "imported"

// Importer implements importer.Importer for curl command lines.
// It is not safe for concurrent use: Warnings reports the last Import.
type Importer struct {
	warnings []string
}

// New returns a curl importer.
func New() *Importer {
	return &Importer{}
}

// Name implements importer.Importer.
func (i *Importer) Name() string { return "curl" }

// CanHandle reports whether the input looks like a curl command line.
func (i *Importer) CanHandle(input string) bool {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), "$"))
	return strings.HasPrefix(trimmed, "curl ") || trimmed == "curl"
}

// Warnings returns non-fatal issues from the last Import.
func (i *Importer) Warnings() []string { return i.warnings }

// Import parses the curl command into a collection with one request.
func (i *Importer) Import(_ context.Context, input string) (*core.Collection, error) {
	req, warnings, err := Parse(input)
	i.warnings = warnings
	if err != nil {
		return nil, err
	}
	return &core.Collection{Name: CollectionName, Requests: []*core.Request{req}}, nil
}

// parser accumulates state while walking curl arguments.
type parser struct {
	req      core.Request
	data     []string
	form     []string
	get      bool
	warnings []string
}

// Parse converts a curl command line into a request plus non-fatal warnings.
func Parse(input string) (*core.Request, []string, error) {
	args, err := tokenize(input)
	if err != nil {
		return nil, nil, fmt.Errorf("curl: tokenize: %w", err)
	}
	if len(args) == 0 || args[0] != "curl" {
		return nil, nil, fmt.Errorf("curl: input does not start with a curl command")
	}

	p := &parser{}
	if err := p.walk(args[1:]); err != nil {
		return nil, p.warnings, err
	}
	req, err := p.finish()
	return req, p.warnings, err
}

// tokenize splits the command shell-style, after folding "\"-newline line
// continuations that Chrome DevTools and man pages produce.
func tokenize(input string) ([]string, error) {
	folded := strings.NewReplacer("\\\r\n", " ", "\\\n", " ").Replace(input)
	folded = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(folded), "$"))
	return shellwords.Parse(folded)
}

func (p *parser) warn(format string, args ...any) {
	p.warnings = append(p.warnings, fmt.Sprintf(format, args...))
}

func (p *parser) walk(args []string) error {
	next := func(i *int, flag string) (string, error) {
		*i++
		if *i >= len(args) {
			return "", fmt.Errorf("curl: flag %s is missing its value", flag)
		}
		return args[*i], nil
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		var value string
		var err error

		switch arg {
		case "-X", "--request":
			if value, err = next(&i, arg); err != nil {
				return err
			}
			p.req.Method = strings.ToUpper(value)

		case "-H", "--header":
			if value, err = next(&i, arg); err != nil {
				return err
			}
			p.addHeader(value)

		case "-d", "--data", "--data-raw", "--data-binary", "--data-ascii":
			if value, err = next(&i, arg); err != nil {
				return err
			}
			if strings.HasPrefix(value, "@") && arg != "--data-raw" {
				p.warn("file body %s skipped (@file is not supported)", value)
				continue
			}
			p.data = append(p.data, value)

		case "--data-urlencode":
			if value, err = next(&i, arg); err != nil {
				return err
			}
			p.data = append(p.data, urlEncodePart(value))

		case "-F", "--form":
			if value, err = next(&i, arg); err != nil {
				return err
			}
			if strings.Contains(value, "@") {
				p.warn("form file field %q skipped (multipart uploads are not supported)", value)
				continue
			}
			p.form = append(p.form, value)

		case "-u", "--user":
			if value, err = next(&i, arg); err != nil {
				return err
			}
			user, pass, _ := strings.Cut(value, ":")
			p.req.Auth = core.Auth{Type: core.AuthBasic, User: user, Pass: pass}

		case "-b", "--cookie":
			if value, err = next(&i, arg); err != nil {
				return err
			}
			p.req.Headers = append(p.req.Headers, core.KV{Name: "Cookie", Value: value})

		case "--url":
			if value, err = next(&i, arg); err != nil {
				return err
			}
			p.req.URL = value

		case "-k", "--insecure":
			p.warn("--insecure ignored: TLS verification stays on")

		case "--compressed":
			// net/http negotiates and decodes gzip transparently.

		case "-G", "--get":
			p.get = true

		default:
			if strings.HasPrefix(arg, "-") {
				p.warn("unknown flag %s ignored", arg)
				continue
			}
			if p.req.URL != "" {
				p.warn("extra argument %q ignored (url already set)", arg)
				continue
			}
			p.req.URL = arg
		}
	}
	return nil
}

// urlEncodePart applies curl's --data-urlencode semantics: "name=value"
// encodes the value, a bare string is encoded whole.
func urlEncodePart(value string) string {
	if name, val, found := strings.Cut(value, "="); found && name != "" {
		return name + "=" + url.QueryEscape(val)
	}
	return url.QueryEscape(strings.TrimPrefix(value, "="))
}

func (p *parser) addHeader(header string) {
	name, value, found := strings.Cut(header, ":")
	if !found {
		p.warn("malformed header %q ignored", header)
		return
	}
	p.req.Headers = append(p.req.Headers, core.KV{
		Name:  strings.TrimSpace(name),
		Value: strings.TrimSpace(value),
	})
}

func (p *parser) finish() (*core.Request, error) {
	if p.req.URL == "" {
		return nil, fmt.Errorf("curl: no URL found in command")
	}

	switch {
	case p.get && len(p.data) > 0:
		// -G turns -d pairs into query parameters.
		for _, part := range p.data {
			for pair := range strings.SplitSeq(part, "&") {
				name, value, _ := strings.Cut(pair, "=")
				p.req.Params = append(p.req.Params, core.KV{Name: name, Value: value})
			}
		}

	case len(p.form) > 0:
		p.warn("multipart form imported as urlencoded body")
		p.req.Body = core.Body{Type: "form", Content: strings.Join(p.form, "&")}

	case len(p.data) > 0:
		p.req.Body = core.Body{Content: strings.Join(p.data, "&")}
		p.req.Body.Type = detectBodyType(&p.req)
	}

	if p.req.Method == "" {
		if !p.req.Body.IsZero() {
			p.req.Method = "POST"
		} else {
			p.req.Method = "GET"
		}
	}

	p.req.Name = requestName(&p.req)
	req := p.req
	return &req, nil
}

func detectBodyType(req *core.Request) string {
	for _, h := range req.Headers {
		if strings.EqualFold(h.Name, "Content-Type") {
			switch {
			case strings.Contains(h.Value, "json"):
				return "json"
			case strings.Contains(h.Value, "x-www-form-urlencoded"):
				return "form"
			default:
				return "text"
			}
		}
	}
	trimmed := strings.TrimSpace(req.Body.Content)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return "json"
	}
	// curl -d defaults to application/x-www-form-urlencoded.
	return "form"
}

// requestName derives a readable tree label like "GET /users/1".
func requestName(req *core.Request) string {
	u, err := url.Parse(req.URL)
	if err != nil || u.Path == "" || u.Path == "/" {
		host := req.URL
		if err == nil && u.Host != "" {
			host = u.Host
		}
		return req.Method + " " + host
	}
	return req.Method + " " + u.Path
}
