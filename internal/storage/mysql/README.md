# mysql storage driver

The bundled storage driver (and the default `DB_DRIVER`). It owns its
entire connection configuration - the app never parses DSNs.

## Environment variables

| Variable | Required | Purpose |
|---|---|---|
| `CONSPECTUS_MYSQL_HOST` | yes | `host:port` or socket; `:3306` appended when the port is omitted |
| `CONSPECTUS_MYSQL_USERNAME` | no | DB user |
| `CONSPECTUS_MYSQL_PASSWORD` | no | DB password |
| `CONSPECTUS_MYSQL_DATABASE` | yes | Database name |

## Notes

- `parseTime=true`, the `utf8mb4` charset and `loc=UTC` are baked into the
  DSN the driver builds; you can't misconfigure them. `loc=UTC` means the
  server session is expected to run UTC (the database image default) - the
  app's calendar time zone (`TZ`) is a display/bucketing concern applied in
  Go, separate from storage.
- Serve startup is gated by `storage.CheckEnv`: with the driver selected
  but its variables missing, the app refuses to start with a diagnostic
  listing exactly what to set. `conspectus help` renders the same contract
  from the driver registry.
- Schema lifecycle is handled by the migrations runner (`migrations/*.sql`
  + `CONSPECTUS_AUTO_MIGRATE`), not by this driver.
