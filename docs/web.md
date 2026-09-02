# The web UI

> Documentation is best efforts - it may lag the code.

The HTML frontend is server-rendered Go `html/template` output. Everything
the UI needs lives in one directory - `web/` in the repo, `/app/web` in the
container - and is read from **disk at runtime** rather than compiled in:

```
web/
  templates/            *.html page templates
    partials/           header, footer, pickers, pagination
    blocks/<page>/      optional sections a page composes in order
    banner.html         the banner strip fragment
  assets/
    css/style.css
    js/app.js           charts, pagers, pickers (progressive enhancement)
    js/vendor/chart.umd.js
```

Point the binary at it with `CONSPECTUS_WEB_DIR` (default `/app/web`,
falling back to `./web` when that path doesn't exist - e.g. running the
bare binary from a checkout). Edit a template or a CSS file and restart the
process to pick the change up; no rebuild is needed. Everything else -
application code, stats engine, even the migration chain - is baked into
the binary, so the deployment unit is *binary + web directory*.

## Rendering model

- Each page handler builds a vars map and renders
  `templates/<page>.html` through the engine (`internal/render`).
- Every template gets the common vars: `Nav`, `Page`, `Version`, `CSRF`,
  `Settings`, `currency_symbol`, `Query` (the `conspectus_ui` cookie as
  query values), `FlashOK`/`FlashErr`, and `Blocks` (below).
- The `conspectus_ui` cookie carries the server-visible UI state (range,
  horizon, window, page size) plus the remembered asset/class scope. A
  scope picked on one browse-picker page re-applies to the others until
  cleared; a scope in the path (`/breakdown/asset/47`) wins over the
  remembered one. While a scope is remembered, the overview redirects to
  that asset/class page - clear the scope to see the whole portfolio.
- Template funcs: `money`, `pct`, `month`, `asset` (asset URL with cache
  buster), `qs`, `qp`, `json` (chart payload → `template.JS`), `raw`,
  `add`, `sub`, `seq`, `pos`, `neg`, and `include` (render a named
  template inline).

## Blocks

Pages with optional sections (overview, asset, class, breakdown, analytics,
projections, manage) compose them from
`templates/blocks/<page>/<block>.html`, in the order the Go code registers
them - analysis sections first, ledger tables last:

```
{{range $b := .Blocks}}{{include (printf "blocks/%s/%s.html" $.Page $b) $}}{{end}}
```

The block lists live in `internal/views/views.go` (analysis blocks) and
`internal/web/pages.go` (core blocks). A block whose template file is
missing is skipped with a warning.

## Assets

`templates` reference assets with the `asset` func:

```
<link rel="stylesheet" href="{{asset "css/style.css"}}">
```

which renders `/assets/css/style.css?v=<sha256-prefix-of-the-file>` - the
version parameter makes browsers re-fetch after an edit+restart. Assets are
served from `web/assets/` with long-lived cache headers.

## Banner

A strip shown above the header on **every page**, for flagging e.g. a
read-only viewer instance. It exists only when the environment variable is
set - **no env var, no banner**:

```
CONSPECTUS_BANNER_TEXT=Read-only viewer · {{version}}
```

- The text is HTML-escaped (it cannot carry markup).
- Three placeholders are substituted, template-style: inner spaces are
  allowed (`{{ version }}`), and anything else in `{{...}}` is left
  untouched.

  | Placeholder | Shows | Example output |
  |---|---|---|
  | `{{version}}` | the version stamp (same one the Manage page shows) | `2.0.0` or `2.0.0-dev-master-e96ebed-dirty` |
  | `{{hostname}}` | the server machine's hostname | `conspectus-app` |
  | `{{date}}` | when the server process started, in server time (UTC by default); fixed for the run, so reloads never change it | `2026-09-02 12:25` |

  So `Read-only viewer · v{{version}} · up since {{ date }}` renders as
  `Read-only viewer · v2.0.0-dev-master-e96ebed-dirty · up since 2026-09-02 12:25`.
- The wrapper markup lives in `templates/banner.html` (currently just
  `<span>{{.Banner}}</span>` inside the styled banner div) - edit it to
  change the banner's shape.
