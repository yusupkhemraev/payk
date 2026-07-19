# payk

**payk** (пайк) — *messenger, courier* in Persian and Tajik. A courier takes
your letter, rides out, and comes back with the reply. That is exactly what
this tool does: a fast, keyboard-first TUI HTTP client that carries your
requests and brings back responses — with timings, pretty JSON, and zero
mouse involved. Think Yaak or the PyCharm HTTP client, living in your
terminal. One static binary, no runtime dependencies.

![payk demo](docs/demo.gif)

## Install

```sh
go install github.com/yusupkhemraev/payk/cmd/payk@latest
```

Homebrew (tap stub, enabled once the tap repo is published):

```sh
brew install yusupkhemraev/tap/payk
```

Or grab a binary from the releases page (darwin/linux/windows,
amd64/arm64).

## Quick start

```sh
cd your-project
mkdir -p .payk/collections/api
payk
```

Requests are plain YAML files — one file per request, folders are
directories, everything git-friendly with stable key order:

```yaml
# .payk/collections/api/create-user.yaml
name: create user
method: POST
url: "{{base_url}}/users"
headers:
  - name: Content-Type
    value: application/json
body:
  type: json
  content: |
    {"name": "Ada"}
auth:
  type: bearer
  token: "{{env:API_TOKEN}}"
```

`{{var}}` comes from the active environment in `.payk/environments.yaml`,
`{{env:NAME}}` from process environment variables. Resolution happens in
memory at send time — secrets are never written to disk.

## Importing

- **curl**: paste any curl command (Chrome DevTools “Copy as cURL” works
  as-is) — payk offers to import it inline.
- **OpenAPI 3.0/3.1**: `:import openapi.yaml`, `:import https://…/openapi.json`,
  or paste the spec. The tree is grouped by tags, request bodies get
  examples generated from schemas, server URLs become environments.
- **FastAPI**: `:import ~/dev/my-fastapi-project` — payk finds the project’s
  interpreter (`.venv`, poetry, uv) and asks the app itself for its schema.
  No Python parsing involved. Set `[tool.payk] entrypoint = "app.main:app"`
  in `pyproject.toml` if the heuristic can’t find your app.

## Keybindings

| Key | Action |
| --- | --- |
| `h` / `l`, `tab` / `shift+tab` | switch pane focus |
| `j` / `k`, `gg` / `G`, `ctrl+d` / `ctrl+u` | move / scroll |
| `enter` | open request · toggle folder · cycle enum field |
| `i` | edit field (insert mode), `esc` back to normal |
| `a` / `d` | add / delete param or header row |
| `d` | delete request/folder/collection in tree (with confirm) |
| `r` | rename in tree · raw/pretty body in response |
| `m` | message log (full text of truncated status messages) |
| `f` | format JSON body |
| `e` | edit body in `$EDITOR` |
| `space` | send request (`esc` cancels) |
| `/`, `n` / `N` | search in pane, next/previous match |
| `w` | wrap long lines |
| `y` | copy response body |
| `z` | fullscreen the focused pane |
| `ctrl+b` | toggle collections sidebar |
| `:` | command line — `:q` `:w` `:send` `:env <name>` `:import <src>` |
| `?` | help overlay |

Typing `{{` in any field suggests variables from the active environment;
`{{env:` completes from process environment names. `tab` accepts,
`ctrl+n/p` cycle.

## Layout

Three panes on wide terminals (collections · request · response), two panes
between 80 and 119 columns with a collapsible sidebar, single pane with tab
switching below 80. Everything stays usable at 60×20.

## Development

```sh
go build ./...   # build
go test ./...    # unit + TUI smoke tests (teatest)
vhs docs/demo.tape  # re-render the demo GIF
```

The domain (`internal/core`), storage, HTTP client, and importers are
plain Go packages with no TUI imports — testable headlessly. Panels are
independent Bubble Tea models; the root model only routes messages.
