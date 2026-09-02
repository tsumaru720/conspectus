# Development environment

> Documentation is best efforts - it may lag the code.

Everything runs through Docker Compose - no Go or database installation on
the host is required.

## The dev stack

The compose file in `dev/` builds the app image locally and runs it against
a MariaDB database. Everything in this section runs from `dev/`:

```sh
cd dev
docker compose up -d --build    # first start (pulls the database image, builds the app)
docker compose logs -f app      # watch the auto-migrate + listen lines
docker compose down             # stop (data volume kept)
docker compose down -v          # stop and wipe the database
```

| Service | Where | Notes |
|---|---|---|
| app | http://127.0.0.1:8090 | Built from `dev/Dockerfile`; host 8080 is avoided on purpose |
| db | MariaDB, `127.0.0.1:13306` | Database `asset_tracker`, user `conspectus`/`conspectus`, root password `devroot` |
| dbadmin | http://127.0.0.1:8091 | Adminer, server `db` |
| go | *(tools profile, never starts with `up`)* | Throwaway Go toolchain - see below |

On the first start (empty volume) the database imports `conspectus.sql` from
the repo root as sample data and the app auto-migrates on top of it.
`down -v` wipes the volume; the next `up` re-imports.

### Change types

The app's bind mounts are development conveniences only - the image ships
everything it needs and runs with none of them:

- **Templates, CSS, JS** - `web/` is bind-mounted over the image's
  `/app/web`: edit a file, `docker compose restart app`, no rebuild.
- **Migrations** - `migrations/` is bind-mounted over the image's
  `/app/migrations` (same version = override). Wipe the database
  volume (`down -v`) to replay the chain from scratch.
- **Go code** - always needs `docker compose up -d --build app`.

## Go build / vet / test

The stack's `go` service is a throwaway toolchain; module and build caches
persist in named volumes. It sets `CONSPECTUS_TEST_MYSQL=1` for you, so a
plain test run includes the MySQL-backed tests against the stack's own
database:

```sh
docker compose run --rm go build ./...
docker compose run --rm go vet ./...
docker compose run --rm go test ./...    # includes the MySQL-backed tests
```

### The `CONSPECTUS_TEST_MYSQL` flag

A test-only opt-in, never read by the app binary. The tests in
`internal/migrations` and `internal/storage` create scratch databases and
run the real migration chain against them; without the flag they SKIP, so a
bare `go test ./...` stays green on any machine with no database at all.
The flag turns them on; even then an unreachable database only skips (the
helpers ping before doing anything), so leaving the flag set is safe.

Where the tests connect:

- **Flag set, no host** (the default here): the stack's own database at
  `db:3306` as `root`/`devroot`. Root, because the helpers must `CREATE`
  and `DROP` scratch databases - the app's unprivileged `conspectus` user
  is refused (Error 1044).
- **`CONSPECTUS_MYSQL_HOST` set**: the flag is implied and the tests target
  that server using the standard `CONSPECTUS_MYSQL_*` variables - the same
  four the app reads. Point them at any external server; its user needs
  create/drop rights for the same reason:

  ```sh
  docker compose run --rm -e CONSPECTUS_MYSQL_HOST=mysql.internal:3306 \
    -e CONSPECTUS_MYSQL_USERNAME=ci -e CONSPECTUS_MYSQL_PASSWORD=... \
    -e CONSPECTUS_MYSQL_DATABASE=asset_tracker \
    go test ./internal/migrations/ ./internal/storage/...
  ```

## Building the image

```sh
docker build -f dev/Dockerfile -t conspectus .   # from the repo root
```

With no `VERSION` build-arg (plain local builds) the Dockerfile derives a
dev version stamp from git; CI passes the release values instead. The stamp
shows on the Manage page and in the banner via `{{version}}`.

## Handy checks

- `curl -s localhost:8090/readyz` - DB reachable + schema current.
- `docker compose exec app conspectus migrate --plan` - what a pending
  migration would apply.
- `docker compose exec db mariadb -uconspectus -pconspectus asset_tracker`
  - SQL shell on the sample data.
