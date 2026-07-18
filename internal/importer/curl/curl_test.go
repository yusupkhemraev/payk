package curl

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yusupkhemraev/payk/internal/core"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func header(req *core.Request, name string) string {
	for _, h := range req.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

func TestParseChromeDevtoolsExport(t *testing.T) {
	req, warnings, err := Parse(fixture(t, "chrome-devtools.curl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	if req.Method != "POST" {
		t.Errorf("data-raw should imply POST, got %q", req.Method)
	}
	if req.URL != "https://api.github.com/graphql" {
		t.Errorf("url = %q", req.URL)
	}
	if req.Name != "POST /graphql" {
		t.Errorf("name = %q", req.Name)
	}
	if got := header(req, "authorization"); got != "Bearer ghp_exampletoken123" {
		t.Errorf("authorization = %q", got)
	}
	if got := header(req, "Cookie"); got != "logged_in=yes; _gh_sess=abc123" {
		t.Errorf("cookie header = %q", got)
	}
	if req.Body.Type != "json" {
		t.Errorf("body type = %q, want json", req.Body.Type)
	}
	if !strings.Contains(req.Body.Content, `"query"`) {
		t.Errorf("body content = %q", req.Body.Content)
	}
}

func TestParseSimpleGet(t *testing.T) {
	req, warnings, err := Parse(fixture(t, "simple-get.curl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if req.Method != "GET" || req.URL != "https://jsonplaceholder.typicode.com/users/1" {
		t.Errorf("got %s %s", req.Method, req.URL)
	}
	if req.Name != "GET /users/1" {
		t.Errorf("name = %q", req.Name)
	}
}

func TestParseFormLoginWithWarnings(t *testing.T) {
	req, warnings, err := Parse(fixture(t, "form-login.curl"))
	if err != nil {
		t.Fatal(err)
	}

	if req.Method != "POST" {
		t.Errorf("method = %q", req.Method)
	}
	if req.Auth.Type != core.AuthBasic || req.Auth.User != "admin" || req.Auth.Pass != "s3cret" {
		t.Errorf("auth = %+v", req.Auth)
	}
	if req.Body.Type != "form" {
		t.Errorf("body type = %q, want form", req.Body.Type)
	}
	if req.Body.Content != "username=admin&note=hello+world+%26+more" {
		t.Errorf("body = %q", req.Body.Content)
	}

	// -k warns, -L and --http1.1 are unknown flags: warn, don't fail.
	joined := strings.Join(warnings, "\n")
	for _, want := range []string{"--insecure", "-L", "--http1.1"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings missing %q: %v", want, warnings)
		}
	}
}

func TestParseGetWithDataAsQueryParams(t *testing.T) {
	req, _, err := Parse("curl -G https://example.com/search -d q=payk -d 'lang=go&limit=5'")
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "GET" {
		t.Errorf("method = %q", req.Method)
	}
	want := []core.KV{{Name: "q", Value: "payk"}, {Name: "lang", Value: "go"}, {Name: "limit", Value: "5"}}
	if !reflect.DeepEqual(req.Params, want) {
		t.Errorf("params = %+v, want %+v", req.Params, want)
	}
	if !req.Body.IsZero() {
		t.Errorf("-G must not produce a body: %+v", req.Body)
	}
}

func TestParseUrlFlagAndExplicitMethod(t *testing.T) {
	req, _, err := Parse("curl -X DELETE --url https://example.com/users/7")
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "DELETE" || req.URL != "https://example.com/users/7" {
		t.Errorf("got %s %s", req.Method, req.URL)
	}
}

func TestParseMultipartFormSkipsFiles(t *testing.T) {
	req, warnings, err := Parse("curl -F name=ada -F 'avatar=@photo.png' https://example.com/upload")
	if err != nil {
		t.Fatal(err)
	}
	if req.Body.Content != "name=ada" {
		t.Errorf("body = %q", req.Body.Content)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "avatar=@photo.png") || !strings.Contains(joined, "urlencoded") {
		t.Errorf("warnings = %v", warnings)
	}
}

func TestParseErrors(t *testing.T) {
	if _, _, err := Parse("curl -H 'X-Only: 1'"); err == nil {
		t.Error("want error when no URL present")
	}
	if _, _, err := Parse("wget https://example.com"); err == nil {
		t.Error("want error for non-curl input")
	}
	if _, _, err := Parse("curl https://example.com -X"); err == nil {
		t.Error("want error for flag missing its value")
	}
}

func TestImporterInterface(t *testing.T) {
	imp := New()
	if !imp.CanHandle("  curl https://example.com") {
		t.Error("CanHandle should accept curl commands")
	}
	if !imp.CanHandle("$ curl https://example.com") {
		t.Error("CanHandle should accept a leading shell prompt")
	}
	if imp.CanHandle("wget https://example.com") || imp.CanHandle("{\"openapi\": \"3.1\"}") {
		t.Error("CanHandle must reject non-curl input")
	}

	col, err := imp.Import(context.Background(), "curl -k https://example.com/a")
	if err != nil {
		t.Fatal(err)
	}
	if col.Name != CollectionName || len(col.Requests) != 1 {
		t.Fatalf("collection = %+v", col)
	}
	if len(imp.Warnings()) != 1 {
		t.Errorf("warnings not reported through the importer: %v", imp.Warnings())
	}
}
