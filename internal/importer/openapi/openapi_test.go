package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusupkhemraev/payk/internal/core"
)

func importFixture(t *testing.T, name string) (*Importer, *core.Collection) {
	t.Helper()
	imp := New()
	col, err := imp.Import(context.Background(), filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return imp, col
}

func folder(t *testing.T, col *core.Collection, name string) *core.Folder {
	t.Helper()
	for _, f := range col.Folders {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("folder %q not found in %+v", name, col.Folders)
	return nil
}

func request(t *testing.T, requests []*core.Request, name string) *core.Request {
	t.Helper()
	for _, r := range requests {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("request %q not found", name)
	return nil
}

func TestImportPetstoreGroupsByTags(t *testing.T) {
	imp, col := importFixture(t, "petstore-3.1.yaml")

	if col.Name != "swagger-petstore" {
		t.Errorf("collection name = %q", col.Name)
	}

	pets := folder(t, col, "pets")
	if len(pets.Requests) != 4 {
		t.Errorf("pets folder has %d requests, want 4", len(pets.Requests))
	}
	store := folder(t, col, "store")
	if len(store.Requests) != 1 {
		t.Errorf("store folder has %d requests, want 1", len(store.Requests))
	}

	// healthz has no tags: grouped by its path prefix.
	healthz := folder(t, col, "healthz")
	if len(healthz.Requests) != 1 {
		t.Errorf("healthz folder has %d requests, want 1", len(healthz.Requests))
	}

	list := request(t, pets.Requests, "List pets")
	if list.Method != "GET" || list.URL != "{{base_url}}/pets" {
		t.Errorf("list pets = %s %s", list.Method, list.URL)
	}
	if len(list.Params) != 1 || list.Params[0].Name != "limit" || list.Params[0].Value != "20" {
		t.Errorf("query params = %+v", list.Params)
	}
	if len(list.Headers) != 1 || list.Headers[0].Name != "X-Request-Id" {
		t.Errorf("header params = %+v", list.Headers)
	}

	if imp.Warnings() != nil {
		t.Errorf("unexpected warnings: %v", imp.Warnings())
	}
}

func TestImportPetstoreGeneratesExampleBodies(t *testing.T) {
	_, col := importFixture(t, "petstore-3.1.yaml")

	create := request(t, folder(t, col, "pets").Requests, "Create a pet")
	if create.Body.Type != "json" {
		t.Fatalf("body type = %q", create.Body.Type)
	}
	body := create.Body.Content
	for _, want := range []string{`"name": "doggie"`, `"status": "available"`, `"email": "user@example.com"`, `"verified": true`} {
		if !strings.Contains(body, want) {
			t.Errorf("example body missing %s:\n%s", want, body)
		}
	}
	if !strings.Contains(body, `"tags": [`) || !strings.Contains(body, `"string"`) {
		t.Errorf("array example missing:\n%s", body)
	}

	order := request(t, folder(t, col, "store").Requests, "Place an order")
	for _, want := range []string{`"quantity": 2`, `"shipDate": "2024-01-01T00:00:00Z"`, `"petId": 0`} {
		if !strings.Contains(order.Body.Content, want) {
			t.Errorf("order body missing %s:\n%s", want, order.Body.Content)
		}
	}
}

func TestImportPetstoreCarriesServersIntoEnvironments(t *testing.T) {
	imp, _ := importFixture(t, "petstore-3.1.yaml")

	envs := imp.Environments()
	if len(envs) != 2 {
		t.Fatalf("environments = %+v", envs)
	}
	if envs[0].Name != "swagger-petstore" || envs[0].Vars["base_url"] != "https://petstore.example.com/v2" {
		t.Errorf("first env = %+v", envs[0])
	}
	if envs[1].Name != "swagger-petstore-2" || envs[1].Vars["base_url"] != "https://staging.petstore.example.com/v2" {
		t.Errorf("second env = %+v", envs[1])
	}
}

func TestImportFastAPISpec(t *testing.T) {
	imp, col := importFixture(t, "fastapi-openapi.json")

	if col.Name != "fastapi" {
		t.Errorf("collection name = %q", col.Name)
	}

	users := folder(t, col, "users")
	read := request(t, users.Requests, "Read Users")
	if len(read.Params) != 2 || read.Params[0].Value != "0" || read.Params[1].Value != "100" {
		t.Errorf("default query params = %+v", read.Params)
	}

	create := request(t, users.Requests, "Create User")
	for _, want := range []string{`"email": "user@example.com"`, `"password": "string"`} {
		if !strings.Contains(create.Body.Content, want) {
			t.Errorf("body missing %s:\n%s", want, create.Body.Content)
		}
	}

	items := folder(t, col, "items")
	item := request(t, items.Requests, "Read Item")
	if item.URL != "{{base_url}}/items/{item_id}" {
		t.Errorf("path params must stay literal: %q", item.URL)
	}

	// "/" root operation has no tag and no path segment: collection root.
	request(t, col.Requests, "Root")

	// FastAPI specs usually have no servers: a warning tells the user to
	// define base_url.
	if len(imp.Warnings()) != 1 || !strings.Contains(imp.Warnings()[0], "no servers") {
		t.Errorf("warnings = %v", imp.Warnings())
	}
	if imp.Environments() != nil {
		t.Errorf("environments = %+v", imp.Environments())
	}
}

func TestImportFromURL(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "petstore-3.1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)

	imp := New()
	col, err := imp.Import(context.Background(), srv.URL+"/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if col.Name != "swagger-petstore" {
		t.Errorf("collection name = %q", col.Name)
	}
}

func TestImportRawContent(t *testing.T) {
	imp := New()
	col, err := imp.Import(context.Background(),
		`{"openapi": "3.0.0", "info": {"title": "Inline"}, "paths": {"/x": {"get": {"summary": "X"}}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if col.Name != "inline" || len(col.Folders) != 1 {
		t.Errorf("collection = %+v", col)
	}
}

func TestCanHandle(t *testing.T) {
	imp := New()
	if !imp.CanHandle("https://example.com/openapi.json") {
		t.Error("URLs must be handled")
	}
	if !imp.CanHandle(filepath.Join("testdata", "petstore-3.1.yaml")) {
		t.Error("existing spec files must be handled")
	}
	if !imp.CanHandle(`{"openapi": "3.1.0"}`) {
		t.Error("raw spec content must be handled")
	}
	if imp.CanHandle("curl https://example.com") || imp.CanHandle("random text") {
		t.Error("non-spec input must be rejected")
	}
	if imp.CanHandle("missing-file.yaml") {
		t.Error("nonexistent files must be rejected")
	}
}

func TestImportRejectsSwagger2(t *testing.T) {
	imp := New()
	_, err := imp.Import(context.Background(), `{"swagger": "2.0", "info": {"title": "Old"}, "paths": {}}`)
	if err == nil {
		t.Fatal("swagger 2.0 must be rejected")
	}
}

func TestImportGroupsNonLatinTags(t *testing.T) {
	spec := `{"openapi": "3.1.0", "info": {"title": "Пайдо API"}, "paths": {
		"/users": {"get": {"summary": "Список пользователей", "tags": ["Пользователи"]}},
		"/files": {"post": {"summary": "Загрузка файла", "tags": ["Файлы"]}}}}`

	imp := New()
	col, err := imp.Import(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if col.Name != "пайдо-api" {
		t.Errorf("collection name = %q", col.Name)
	}
	folder(t, col, "пользователи")
	folder(t, col, "файлы")
	if len(col.Folders) != 2 {
		t.Errorf("folders = %+v, want two distinct tag folders", col.Folders)
	}
}

func TestImportAppliesSecuritySchemes(t *testing.T) {
	spec := `{
	  "openapi": "3.1.0", "info": {"title": "Secured"},
	  "security": [{"bearerAuth": []}],
	  "paths": {
	    "/private": {"get": {"summary": "Private", "tags": ["a"]}},
	    "/basic": {"get": {"summary": "Basic", "tags": ["a"],
	      "security": [{"basicAuth": []}]}},
	    "/keyed": {"get": {"summary": "Keyed", "tags": ["a"],
	      "security": [{"keyAuth": []}]}},
	    "/open": {"get": {"summary": "Open", "tags": ["a"], "security": []}}
	  },
	  "components": {"securitySchemes": {
	    "bearerAuth": {"type": "http", "scheme": "bearer"},
	    "basicAuth": {"type": "http", "scheme": "basic"},
	    "keyAuth": {"type": "apiKey", "in": "header", "name": "X-Api-Key"}
	  }}}`

	imp := New()
	col, err := imp.Import(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	requests := folder(t, col, "a").Requests

	private := request(t, requests, "Private")
	if private.Auth.Type != core.AuthBearer || private.Auth.Token != "{{api_token}}" {
		t.Errorf("global bearer not applied: %+v", private.Auth)
	}

	basic := request(t, requests, "Basic")
	if basic.Auth.Type != core.AuthBasic || basic.Auth.User != "{{api_user}}" || basic.Auth.Pass != "{{api_password}}" {
		t.Errorf("operation basic override not applied: %+v", basic.Auth)
	}

	keyed := request(t, requests, "Keyed")
	found := false
	for _, h := range keyed.Headers {
		if h.Name == "X-Api-Key" && h.Value == "{{api_key}}" {
			found = true
		}
	}
	if !found {
		t.Errorf("apiKey header not applied: %+v", keyed.Headers)
	}
	if keyed.Auth.Type != core.AuthNone {
		t.Errorf("apiKey must not set auth: %+v", keyed.Auth)
	}

	// Explicit empty security disables the global requirement.
	open := request(t, requests, "Open")
	if open.Auth.Type != core.AuthNone {
		t.Errorf("empty security should mean no auth: %+v", open.Auth)
	}

	joined := strings.Join(imp.Warnings(), "\n")
	for _, want := range []string{"api_token", "api_user", "api_password", "api_key"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warning should list %s: %v", want, imp.Warnings())
		}
	}
}
