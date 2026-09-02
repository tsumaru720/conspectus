# Conspectus

> **Built with AI assistance.** This project was developed with the help of
> AI coding agents (GLM, via ZCode) working from design briefs and review
> feedback. Documentation is best efforts - it may lag the code.

A personal wealth tracker. One small Go binary providing a JSON API and a
server-rendered web UI over a MySQL schema - monthly snapshots of what you
own, what you've put in, and how it's doing.

You record, per asset and per month, two numbers: the **cumulative deposits**
(everything you've ever paid in) and the **current value** (what it's worth
now). Income events (dividends, interest) are recorded separately as
**payments**. Everything else - returns, growth rates, time-weighted rate of
return, projections, breakdowns - is derived.

## The pages

| Page | What it shows |
|---|---|
| **Overview** (`/`) | KPI cards (value, deposits, return, TWRR, CAGR) with 1m/1y deltas, value-vs-deposits, return and TWRR charts, monthly changes |
| **Breakdown** (`/breakdown`) | Allocation by class and by asset (pies, over-time stacks), class table, closed-position summary |
| **Analytics** (`/analytics`) | Yearly table (opening/deposits/growth/income/end), records, monthly-return histogram, year×month heatmap, new-money-vs-growth decomposition |
| **Projections** (`/projections`) | Monte-Carlo wealth paths (p10–p90 band + median + CAGR line), milestone targets, required-pace target curve, actual-vs-target track record |
| **Ledger** (`/ledger`) | Every log entry and payment, searchable, paginated, inline edit/delete, quick payment form |
| **Asset** (`/asset/47`) | A single asset: its details, full payment history, and the analysis blocks (KPIs, charts, income) scoped to it |
| **Class** (`/class/3`) | A single class: payments across its assets and the same analysis scoped to the class |
| **Import** (`/import`) | Quick-update grid (one row per asset, prefilled with last month) and two-phase CSV import |
| **Manage** (`/manage`) | Assets, classes, settings (projection targets), system info |

Scope any of the data pages to a single asset or class via the picker -
analytics for `/analytics/asset/47`, and so on.

## Architecture

- **API-first.** The API half owns storage, the statistics engine and
  settings. The frontend half (pages, analysis views, CSV importer) consumes
  data *exclusively* over HTTP - in bundled deployments over an in-process
  loopback client, in split deployments against a remote API server. The
  frontend is just another API client.
- **Everything compiled in.** All pages - data management (ledger, import,
  manage, asset/class) and the analysis views (overview, breakdown,
  analytics, projections) - are plain compiled Go
  (`internal/web`, `internal/views`). The stats endpoints live in the API
  half (`internal/httpapi`). No plugin system, no interpreter.
- **Templates and assets are files, not embedded.** The HTML templates,
  CSS/JS assets and the banner fragment ship next to the binary
  (`web/`, pointed at by `CONSPECTUS_WEB_DIR`) so they can be edited in
  place - a restart picks them up, no rebuild. Everything else (including
  the migration chain) is baked into the binary. See
  [docs/web.md](docs/web.md).
- **No build step for the web UI**; no JS required for core flows (forms
  POST and redirect; JS only enhances charts, pagers and pickers).

## Running the published image

The release workflow publishes multi-arch images to GHCR on every release:
`ghcr.io/tsumaru720/conspectus` tagged with the release version, plus
`latest`. After the first release, set the package visibility to public in
the repo's package settings so it can be pulled without authentication.

### `docker run` (you already have a MySQL/MariaDB)

```sh
docker run -d --name conspectus \
  -p 8080:8080 \
  -e CONSPECTUS_MYSQL_HOST=mysql.internal:3306 \
  -e CONSPECTUS_MYSQL_USERNAME=conspectus \
  -e CONSPECTUS_MYSQL_PASSWORD='s3cret' \
  -e CONSPECTUS_MYSQL_DATABASE=asset_tracker \
  -e TZ=Europe/London \
  ghcr.io/tsumaru720/conspectus:latest
```

The database can be completely empty - the app builds the schema itself on
first boot. `CONSPECTUS_MYSQL_HOST` is the only address you configure; the
driver assembles the connection (default port 3306, `parseTime`, utf8mb4).
No volumes are needed beyond the database's own: templates, assets and
migrations all ship inside the image, and all application data lives in the
database.

### `docker compose` (with a throwaway database)

The bundled database is optional - drop the `db` service and point
`CONSPECTUS_MYSQL_HOST` at your own server if you already have one.

```yaml
services:
  db:
    image: mariadb:latest
    environment:
      MARIADB_DATABASE: ${DATABASE:-asset_tracker}
      MARIADB_USER: ${DB_USERNAME:-conspectus}
      MARIADB_PASSWORD: ${DB_PASSWORD:-change-me}
      MARIADB_ROOT_PASSWORD: ${DB_ROOT_PASSWORD:-change-me-too}
    volumes: [db-data:/var/lib/mysql]
    healthcheck:
      test: ["CMD", "healthcheck.sh", "--connect", "--innodb_initialized"]
      interval: 10s
      timeout: 5s
      retries: 10

  app:
    image: ghcr.io/tsumaru720/conspectus:latest
    depends_on:
      db: { condition: service_healthy }
    environment:
      CONSPECTUS_MYSQL_HOST: db:3306
      CONSPECTUS_MYSQL_USERNAME: ${DB_USERNAME:-conspectus}
      CONSPECTUS_MYSQL_PASSWORD: ${DB_PASSWORD:-change-me}
      CONSPECTUS_MYSQL_DATABASE: ${DATABASE:-asset_tracker}
      TZ: ${TZ:-UTC}
    ports: ["8080:8080"]
    restart: unless-stopped

volumes:
  db-data:
```

### Importing a dump from an existing database

To seed a fresh database container from an existing Conspectus (v1 or v2)
dump, mount it into the database's first-boot import directory:

```yaml
  db:
    image: mariadb:latest
    ...
    volumes:
      - db-data:/var/lib/mysql
      - ./backup.sql:/docker-entrypoint-initdb.d/01-backup.sql:ro
```

The entrypoint imports `/docker-entrypoint-initdb.d/*.sql` once, on first
boot with an empty data volume. The dump's `settings` table carries
`db_version`, so the app resumes the migration chain from there (v1 dumps
land at 5; migration 0006 applies on top). Make the dump with:

```sh
mysqldump --single-transaction asset_tracker > backup.sql
```

The volume must be empty for the import to run - `docker compose down -v`
first if you need to redo it. To import into a database the container has
already initialised, use `docker exec -i <db-container> mysql -uconspectus -p asset_tracker < backup.sql` instead.

## Building and running from source

The example compose file at the repo root (`docker-compose-example.yaml`)
runs the published registry image. Compose does not auto-load that name, so
the root slot stays free for your own `docker-compose.yaml` - copy the
example there; it is yours to edit and to keep out of git:

```sh
cp docker-compose-example.yaml docker-compose.yaml
cp .env.example .env   # edit passwords
docker build -f dev/Dockerfile -t conspectus .   # optional: your own build
# in docker-compose.yaml: image: conspectus
docker compose up -d
# app on :8080, empty schema created automatically
```

Bare binary:

```sh
go build -o conspectus ./cmd/conspectus
```

For day-to-day development (live-editable templates, sample data, running
tests against MariaDB) use the dev stack in `dev/` - see
[docs/dev.md](docs/dev.md).

Upgrading an existing Conspectus v1 database? Point v2 at it and start -
pending migrations apply automatically on start. Prefer to drive it
yourself: run `conspectus migrate` first and set `CONSPECTUS_AUTO_MIGRATE=false`.
The migration chain matches v1's numbering 1:1, so it resumes from your
database's stored `db_version` and applies only what's missing. Back up first:

```sh
mysqldump --single-transaction asset_tracker > backup.sql
```

## CLI

```
conspectus                    start the HTTP server (API + web UI; the default)
conspectus migrate            apply pending database migrations
conspectus migrate --plan     dry-run: list pending migrations
conspectus migrate --repair   resume a failed migration from its recorded offset
conspectus version            build info + expected schema version
```

## Environment variables

All variables are prefixed `CONSPECTUS_`. Boolean values accept
`1/true/yes/on` and `0/false/no/off`. The **calendar time zone** for month
bucketing and date display comes from the standard `TZ` variable (UTC when
unset) - set the same variable on the database container, which also runs
UTC by default.

### Core

| Variable | Default | Purpose |
|---|---|---|
| `CONSPECTUS_DB_DRIVER` | `mysql` | Storage driver (drivers are pluggable; `mysql` is compiled in) |
| `CONSPECTUS_AUTO_MIGRATE` | `true` | Apply pending schema migrations at startup. `false` = refuse to serve a stale schema until `conspectus migrate` is run explicitly |
| `CONSPECTUS_READ_ONLY` | `false` | Hard lock: refuse every mutation, including the settings API. There is no database-level escape hatch |
| `CONSPECTUS_WEB` | `true` | `false` = API-only mode: no HTML pages, non-API paths 404 |
| `CONSPECTUS_API_URL` | *(unset)* | Split deployment: point this process's frontend at a remote API base URL. No local database needed or opened; core CRUD routes are not registered locally |
| `CONSPECTUS_WEB_DIR` | `/app/web` | Directory holding the web UI's `templates/` and `assets/` (falls back to `./web` when the container path is absent) |
| `CONSPECTUS_MIGRATIONS_DIR` | `/app/migrations` | Directory of SQL migrations merged over the embedded set (same version = override); bind-mount over it to override in a container |
| `CONSPECTUS_LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error` |
| `CONSPECTUS_LOG_FORMAT` | `text` | `text` \| `json` |
| `CONSPECTUS_LOG_COLOURS` | `true` | Colourize text logs (log level, HTTP method, status code). `false` = plain text; ignored for `json` |

### MySQL driver

The driver builds its own connection from these (no DSN):

| Variable | Default | Purpose |
|---|---|---|
| `CONSPECTUS_MYSQL_HOST` | — (required) | Host, optionally `host:port` (default port 3306) |
| `CONSPECTUS_MYSQL_USERNAME` | — | Database user |
| `CONSPECTUS_MYSQL_PASSWORD` | — | Database password |
| `CONSPECTUS_MYSQL_DATABASE` | — (required) | Database name |

The server session time zone should be UTC - the default for the MariaDB
and MySQL container images.

### Auth and banner

| Variable | Purpose |
|---|---|
| `CONSPECTUS_BASICAUTH_USERS` | `user:sha256hex[,user:sha256hex…]` - hash with `printf '%s' 'password' \| sha256sum`. Inert until set; env wins over the `basicauth_users` setting |
| `CONSPECTUS_BASICAUTH_TOKENS` | `sha256hex[,sha256hex…]` bearer tokens (CSRF-exempt API access). Env wins over the `basicauth_tokens` setting |
| `CONSPECTUS_BANNER_TEXT` | Show a banner strip on every page with this text (HTML-escaped). `{{version}}` (the version stamp, e.g. `2.0.0`), `{{hostname}}` (the server's hostname) and `{{date}}` (the process start time, e.g. `2026-09-02 12:25`, fixed until restart) are substituted; spaces inside the braces are allowed, e.g. `{{ version }}`. **No env var = no banner.** The wrapper markup lives in `web/templates/banner.html`; details and examples in [docs/web.md](docs/web.md). There is deliberately no database setting - the banner is per-instance, and instances often share a database |

## Deployment modes

| Mode | How | Notes |
|---|---|---|
| **Bundled** | *(default)* | One process: API + pages, frontend talks to the API over an in-process loopback (full HTTP semantics, no socket) |
| **API-only** | `CONSPECTUS_WEB=false` | JSON API + `/healthz` + `/readyz` only; pages disabled |
| **Frontend-only** | `CONSPECTUS_API_URL=http://api:8080` | Pages only, no database; all data fetched from the remote API. Pair with an API-only process for a clean two-process split |

## Documentation

- [Quickstart](docs/quickstart.md) - from nothing to running, including v1 upgrades
- [API reference](docs/api.md) - every endpoint with request/response examples
- [Web UI](docs/web.md) - templates, assets, blocks, banner
- [Development](docs/dev.md) - dev stack, local image builds, running the tests

## Tests

```sh
go test ./...
```

The migration and MySQL storage tests need a reachable database and skip
otherwise. With the dev stack up they run on its compose network - see
[docs/dev.md](docs/dev.md) for the command - or point them at any server
with the standard `CONSPECTUS_MYSQL_*` variables plus
`CONSPECTUS_TEST_MYSQL=1`.
