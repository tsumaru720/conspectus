# Quickstart

> Documentation is best efforts - it may lag the code.

From nothing to a running Conspectus. Docker Compose from a checkout is the
main path; the published image and a bare binary follow, plus upgrading an
existing Conspectus v1 database.

The application is happy on a **completely empty database**: it creates the
schema itself, the API returns empty-but-valid responses, and every page
renders with "no data yet" states. You do not need to pre-seed anything.

---

## 1. Docker Compose (recommended)

```sh
git clone <this repo> conspectus && cd conspectus
cp docker-compose-example.yaml docker-compose.yaml
cp .env.example .env
# edit .env: set DB_PASSWORD and DB_ROOT_PASSWORD
docker compose up -d
```

That starts:

- **app** on `http://localhost:8080` (API + web UI)
- **db** - a MariaDB database with an empty `asset_tracker` database

On first boot the app builds the schema automatically (watch
`docker compose logs -f app` for `auto-migrate complete`). Open
`http://localhost:8080` and start recording data.

The database volume is the only storage that matters: the web UI and
migration chain ship inside the image, and all application data lives in
the database. Editing templates or CSS is a development concern - see
[dev.md](dev.md) for the live-edit setup.

## 2. The published image

`docker run` and a compose example against the GHCR image - see the
README's ["Running the published image"](../README.md#running-the-published-image)
section.

## 3. Bare binary

You need Go and a MySQL or MariaDB server you can reach.

```sh
go build -o conspectus ./cmd/conspectus

mysql -u root -p -e "CREATE DATABASE asset_tracker CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
# optional restricted user:
#   CREATE USER 'conspectus'@'%' IDENTIFIED BY '...';
#   GRANT ALL ON asset_tracker.* TO 'conspectus'@'%';

export CONSPECTUS_MYSQL_HOST=localhost:3306
export CONSPECTUS_MYSQL_USERNAME=conspectus
export CONSPECTUS_MYSQL_PASSWORD=...
export CONSPECTUS_MYSQL_DATABASE=asset_tracker
export TZ=Europe/London

./conspectus                     # http://localhost:8080
```

Notes:

- Run the binary from the repo root (or set `CONSPECTUS_WEB_DIR`) so the
  `web/` templates and assets are found; without them the API works but
  HTML pages fail to render.
- `./conspectus migrate --plan` shows what would be applied without
  touching anything.
- Migrations are resumable: each statement's position is bookmarked, and
  `./conspectus migrate --repair` resumes a half-applied file after a crash.

## 4. Upgrading a Conspectus v1 database

v2 uses the **same migration numbering** as v1 (v1's chain is migrations
1–5; v2 adds 6+). A v1 database at, say, `db_version=3` upgrades in place -
v2 applies migrations 4, 5, 6 and nothing else. Data is never touched. Schema 6 is the same version the final v1 build shipped, so a fully-updated v1 database and a v2 database are interchangeable.

```sh
# 1. back up (seriously)
mysqldump --single-transaction asset_tracker > backup-$(date +%F).sql

# 2. point v2 at the same database (same env vars as above)

# 3. start - pending migrations apply automatically on start.
#    `conspectus migrate` applies them explicitly instead;
#    CONSPECTUS_AUTO_MIGRATE=false turns the automatic path off.
```

What happens to an old dump imported into a fresh MySQL (a common v1
migration path) works identically: the `db_version` row in `settings` says
where to resume.

## First data

1. **Manage → Add class** - e.g. "Savings", "Shares".
2. **Manage → Add asset** - e.g. "Bank savings", "Vanguard ISA", class + name.
3. **Import → quick grid** - one row per asset: set this month's cumulative
   deposits and current value; optionally a payment received this month.
   Submit monthly. Unchanged rows can be skipped with one checkbox.
   Or use the CSV path: `id,name,deposit,value` rows (paste from a
   spreadsheet), preview, commit.
4. Or drive it entirely over the API - see [api.md](api.md):

```sh
curl -X POST localhost:8080/api/v1/classes \
  -H 'Content-Type: application/json' \
  -d '{"description":"Savings"}'

curl -X POST localhost:8080/api/v1/assets \
  -H 'Content-Type: application/json' \
  -d '{"class_id":1,"description":"Bank savings"}'

curl -X POST localhost:8080/api/v1/logs \
  -H 'Content-Type: application/json' \
  -d '{"date":"2026-08-01","entries":[{"asset_id":1,"deposit":"1000.00","value":"1005.00"}]}'
```

## Everyday operation

- **Monthly cycle**: visit Import, update values, submit. That's the whole
  ritual.
- **Income**: record dividends/interest the day they land (quick payment
  form on the Ledger, or `POST /api/v1/payments`).
- **Closing an asset** (sold, closed account): Manage → Close. History stays
  in the wealth picture; the asset stops being updatable.
- **Settings**: Manage → settings - the projection-targets card, plus any
  setting row that opts in via its `display` flag.

## Read-only viewer (optional)

A second instance pointed at the same database with
`CONSPECTUS_READ_ONLY=true` serves a fully functional view that cannot
write anything

## Two-process split (optional)

```sh
# process A: pure API (with the database)
CONSPECTUS_WEB=false conspectus

# process B: pages only, no database
CONSPECTUS_API_URL=http://api-host:8080 conspectus
```

Process B registers only page routes; all data flows over HTTP from A.
