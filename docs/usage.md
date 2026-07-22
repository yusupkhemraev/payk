# payk — user guide

payk is a keyboard-first HTTP client for the terminal. This guide walks
through everything it does; the [README](../README.md) has the short
version and the keybinding table.

## 1. Workspace

A workspace is a `.payk/` directory:

```
.payk/
├── environments.yaml
└── collections/
    ├── api/                  ← collection
    │   ├── users/            ← folder
    │   │   ├── list-users.yaml
    │   │   └── create-user.yaml
    │   └── ping.yaml         ← request
    └── admin/
        └── stats.yaml
```

On start payk walks up from the current directory looking for `.payk/`,
then falls back to `~/.config/payk`. If nothing exists, the first import
creates `./.payk` for you. One YAML file per request keeps git diffs
minimal; renames show up as file moves.

## 2. Moving around

The UI is three panes: **Collections** (tree), **Request** (editor),
**Response** (viewer). Focus follows vim keys:

- `h` / `l` or `tab` / `shift+tab` — move focus between panes;
- `j` / `k`, `gg` / `G`, `ctrl+d` / `ctrl+u` — move and scroll inside a pane;
- `z` — fullscreen the focused pane (press again to restore);
- `<` / `>` — narrow / widen the focused pane;
- `ctrl+b` — hide/show the collections sidebar (two-pane layouts);
- `?` — help overlay, adapts to the terminal and scrolls if needed.

The status bar at the bottom shows the focused pane, the active
environment, and transient messages. Long messages are truncated — press
`m` (or `:messages`) to read the full text of everything recent.

## 3. The collections tree

- `enter` on a folder toggles it; on a request, loads it into the editor.
- `a` creates a new request in the selected collection or folder: type a
  name, press `enter` — an empty GET opens in the editor, ready for a URL.
  In an empty workspace the request lands in a new `api` collection.
- Folders start collapsed; every node shows its request count.
- `/` filters the tree live — by name, method, URL, and path. Matches show
  flat with their location; `enter` jumps to the match in the full tree,
  `esc` cancels.
- `r` renames the selected request, folder, or collection (files and
  directories move on disk accordingly).
- `d` deletes the selected node after a `y/n` confirmation.

## 4. Editing a request

Focus the editor and move between its tabs with `[` and `]`:
**URL · Params · Headers · Body · Auth**.

The editor is modal: `j`/`k` select a row, `i` or `enter` acts on it,
`esc` returns to normal mode and commits the value.

- **URL** — `enter` on the method row cycles GET → POST → …; `i` on the
  url/desc rows edits them.
- **Params / Headers** — `a` adds a row, `d` deletes one. While editing,
  `tab` jumps between name and value, and `enter` commits the row and
  opens a fresh one, so several entries can be typed in a chain.
- **Body** — `enter` on the type row cycles json/text/form; `i` on content
  opens inline editing, `e` opens the body in `$EDITOR` (payk suspends,
  you edit, save, quit — the body comes back). `f` pretty-prints valid
  JSON. In normal mode JSON bodies render syntax-highlighted.
- **Auth** — `enter` on the type row cycles none/bearer/basic; `i` edits
  token or user/pass.

`:w` saves the request back to its YAML file — placeholders are stored
as-is, never resolved values.

### Variable completion

Type `{{` in any field and payk suggests variables from the active
environment; type `{{env:` to complete from process environment names.
`tab` accepts the highlighted suggestion, `ctrl+n`/`ctrl+p` cycle.

## 5. Environments and variables

`environments.yaml` holds named variable sets:

```yaml
active: local
environments:
  - name: local
    vars:
      base_url: http://localhost:8000
  - name: staging
    vars:
      base_url: https://staging.example.com
```

- `{{base_url}}` anywhere in a request resolves from the active
  environment at send time;
- `{{env:API_TOKEN}}` resolves from the process environment — good for
  secrets that must never land in a file;
- unresolved variables block the send with an error naming them — nothing
  goes out half-substituted.

Manage environments without touching the file:

```
:env                       list environments (* marks the active one)
:env new staging           create an environment and switch to it
:env staging               switch (persisted in environments.yaml)
:set base_url http://x     set a variable in the active environment
:set token=abc             name=value form works too
:unset token               remove a variable
```

`:set` with no environments yet bootstraps a `default` one, and freshly
set variables immediately show up in `{{` completion.

## 6. Sending and reading responses

`space` (or `:send`) sends the request in the editor. A spinner shows
in-flight state; `esc` cancels. The response viewer has four tabs:

- **Body** — JSON is pretty-printed and syntax-highlighted (capped for
  huge payloads). `r` toggles the raw, unformatted bytes. `/` searches
  incrementally, `n`/`N` cycle matches with the current line highlighted.
  `w` soft-wraps long lines. `y` copies the body to the clipboard
  (OSC52 — works over ssh). The status line shows the HTTP status,
  protocol, size, total time, and your scroll position.
- **Headers** — aligned name/value table, scrollable, wrappable.
- **Timings** — DNS, TCP connect, TLS, TTFB, and total, plus size and
  status. Phases that did not happen show as `—`.
- **History** — every send of the session (last 50), newest first, with
  time, status, and duration. `j`/`k` select, `enter` loads that past
  response back into the viewer. Errors are recorded too.

## 7. Importing

### curl

Copy a request as cURL (Chrome DevTools → Network → Copy as cURL) and
paste it into payk. A prompt appears — `y` imports it into the
`imported` collection and saves it to disk. Multi-line commands with
`\` continuations parse fine; unknown flags produce warnings, not
failures.

### OpenAPI 3.0 / 3.1

```
:import ./openapi.yaml
:import https://api.example.com/openapi.json
```

or just paste the spec JSON. The importer:

- groups operations into folders by their first tag (falling back to the
  first path segment) — any script works, «Пользователи» stays
  «пользователи»;
- names requests from `summary`, then `operationId`;
- generates example JSON bodies from schemas (examples/defaults/enums
  first, then type-driven) and fills query/header parameters with their
  defaults;
- turns server URLs into environments carrying `base_url`, merged into
  `environments.yaml` without touching your other variables;
- applies declared security: HTTP bearer → bearer auth with
  `{{api_token}}`, HTTP basic → `{{api_user}}`/`{{api_password}}`,
  `apiKey` → a header or query parameter with `{{api_key}}`,
  oauth2/OIDC → bearer. The placeholders stay unresolved until you define
  them, so a send fails loudly instead of going out unauthenticated —
  a warning after import lists exactly which variables to set.

### Re-importing

Spec and FastAPI imports record their source in the collection
(`collections/<name>/.import`). When the API changes:

```
:reimport paydo-api        refresh one collection from its source
:reimport                  refresh every imported collection
```

A re-import replaces the collection's contents, so new routes appear and
removed or renamed ones disappear — local edits to imported requests are
replaced along with them. curl imports are not affected: pasted commands
keep accumulating in `imported`.

### FastAPI

```
:import ~/dev/my-fastapi-project
```

payk finds the project's interpreter (`.venv/bin/python`, then
`poetry env info -e`, then `uv run python`), locates the app
(`[tool.payk] entrypoint = "app.main:app"` in `pyproject.toml`, else a
scan of `main.py`/`app.py` variants), runs `app.openapi()` in a
subprocess with a 30s timeout, and feeds the result to the OpenAPI
importer. If the app fails to import, the Python stderr is shown so you
can see the actual traceback.

## 8. Command line

`:` opens the command line (pasting into it works):

| Command | Action |
| --- | --- |
| `:q` | quit |
| `:w` | save the current request to disk |
| `:send` | send the current request |
| `:env` | list environments |
| `:env new <name>` | create an environment and switch to it |
| `:env <name>` | switch the active environment |
| `:set <name> <value>` | set a variable in the active environment |
| `:unset <name>` | remove a variable from the active environment |
| `:import <file-or-url-or-dir>` | run an importer (`~` expands) |
| `:reimport [collection]` | re-run recorded import sources |
| `:messages` | open the message log |
