# Conspectus API

> Documentation is best efforts - it may lag the code.

Every data surface in Conspectus is an HTTP JSON API - the web UI itself is
just a client of it. Base URL: `http://<host>:8080`.

- [Conventions](#conventions)
- [Health & system](#health--system)
- [Assets](#assets)
- [Classes](#classes)
- [Log entries](#log-entries)
- [Payments](#payments)
- [Settings](#settings)
- [Statistics](#statistics)
- [UI row feeds](#ui-row-feeds)

All examples use made-up data.

## Conventions

**Envelope.** Successes wrap the payload; collections add `meta`:

```json
{ "data": { ... } }
{ "data": [ ... ], "meta": { "page": 1, "per_page": 50, "total": 12 } }
```

Errors always look like:

```json
{ "error": { "code": "validation_error", "message": "date is required (YYYY-MM-DD)",
             "details": ["deposit"] } }
```

| HTTP | `code` | Meaning |
|---|---|---|
| 400 | `validation_error` | Bad input (unknown JSON fields and multiple objects are rejected) |
| 401 | `unauthorized` | Auth gate rejected the request (basic auth / bearer token) |
| 403 | `readonly` | Read-only mode refused a mutation |
| 403 | `unauthorized` | CSRF token missing/invalid on a browser-session mutation |
| 404 | `not_found` | Unknown id or path |
| 409 | `conflict` | Duplicate (asset-month entry, asset name in class) or referential block |
| 500 | `internal` | Unexpected failure (logged server-side) |

**Money** is a decimal string with at most 2 dp - `"1234.56"`, `"0.00"`,
`"-20.00"`. Parsing is lenient on the way *in* (commas, `£`/`$`/`€`/`¥`,
spaces are stripped: `"£1,234.50"` is fine) and canonical on the way out.

**Dates** are `YYYY-MM-DD` (interpreted in the app's calendar time zone -
the `TZ` variable, UTC by default) or full
RFC3339. Future dates are rejected. **Months** are `YYYY-MM`.

**Mutations & CSRF.** Plain API clients (no cookies) work untouched. Once a
browser session holds the `conspectus_csrf` cookie, mutations must echo it
in `X-CSRF-Token` (or a `csrf_token` form field). Requests with an
`Authorization` header (bearer token) are exempt.

**Pagination.** List endpoints accept `page` (1-based) and `per_page`
(1–500, default 50). Filters listed per endpoint below.

## Health & system

### `GET /healthz`

Liveness. `200` plain text:

```
ok
```

### `GET /readyz`

Readiness: DB connectivity + schema version. `503` with
`"status":"unready"` when not ready:

```json
{ "status": "ok",
  "checks": { "db": { "ok": true },
              "migrations": { "ok": true, "current": "6" } } }
```

### `GET /api/v1/system`

```json
{ "data": { "version": "2.0.0", "commit": "9f3ab2c",
    "expected_schema_version": 6, "schema_version": "6",
    "read_only": false,
    "routes": 79 } }
```

## Assets

### `GET /api/v1/assets`

Query: `q` (substring) *or* `regex` (RE2) on description; `class_id`;
`closed=true|false`; `sort` (`class,description` default, `description`,
`-description`, `id`, `-id`); `page`, `per_page`.

```json
{ "data": [
    { "id": 1, "class_id": 1, "description": "Bank savings", "closed": false },
    { "id": 2, "class_id": 2, "description": "Vanguard ISA", "closed": false },
    { "id": 7, "class_id": 2, "description": "Apple Inc.", "closed": true }
  ],
  "meta": { "page": 1, "per_page": 50, "total": 3 } }
```

### `POST /api/v1/assets`

```json
{ "class_id": 1, "description": "Premium bonds" }
```

`201`:

```json
{ "data": { "id": 8, "class_id": 1, "description": "Premium bonds", "closed": false } }
```

Duplicate name within a class → `409 conflict`.

### `GET /api/v1/assets/{id}`

```json
{ "data": { "id": 2, "class_id": 2, "description": "Vanguard ISA", "closed": false } }
```

### `PATCH /api/v1/assets/{id}`

Partial update - omit fields to keep them:

```json
{ "description": "Vanguard ISA (moved)", "class_id": 3, "closed": false }
```

### `DELETE /api/v1/assets/{id}[?force=true]`

Refuses while history exists:

```json
{ "error": { "code": "conflict",
    "message": "asset has 42 log entries and 9 payments; retry with force=true to cascade-delete them",
    "log_entries": 42, "payments": 9 } }
```

With `force=true`:

```json
{ "data": { "deleted": true, "id": 7, "log_entries": 42, "payments": 9 } }
```

### `POST /api/v1/assets/{id}/close` · `POST /api/v1/assets/{id}/reopen`

Both return the updated asset:

```json
{ "data": { "id": 7, "class_id": 2, "description": "Apple Inc.", "closed": true } }
```

## Classes

### `GET /api/v1/classes`

```json
{ "data": [ { "id": 1, "description": "Cash" },
            { "id": 2, "description": "Shares" },
            { "id": 3, "description": "Crypto" } ] }
```

### `POST /api/v1/classes`

```json
{ "description": "Bonds" }
```

### `GET/PATCH/DELETE /api/v1/classes/{id}`

`DELETE` returns `409 conflict` while assets reference the class.

## Log entries

A log entry is one asset's monthly snapshot: cumulative deposits + current
value. One entry per asset per month (enforced).

### `GET /api/v1/logs`

Query: `asset_id`, `class_id`, `from`/`to` (`YYYY-MM`), `q`/`regex` on asset
description, `sort` (`-epoch` default, `epoch`, `asset`, `-asset`, `value`,
`-value`), `page`, `per_page`.

```json
{ "data": [
    { "id": 412, "asset_id": 2, "at": "2026-08-31T00:00:00Z",
      "deposit": "18250.00", "value": "21406.35" },
    { "id": 411, "asset_id": 1, "at": "2026-08-31T00:00:00Z",
      "deposit": "24000.00", "value": "24012.80" }
  ],
  "meta": { "page": 1, "per_page": 50, "total": 412 } }
```

### `POST /api/v1/logs`

The month-snapshot write endpoint: one date, one row per asset, optional
payment per row, and a duplicate policy.

```json
{ "date": "2026-08-01",
  "on_duplicate": "replace",
  "entries": [
    { "asset_id": 1, "deposit": "24000.00", "value": "24012.80", "payment": "3.20" },
    { "asset_id": 2, "deposit": "18250.00", "value": "21406.35" },
    { "asset_id": 7, "deposit": "1500.00", "value": "1500.00" }
  ] }
```

`on_duplicate`: `"skip"` (default) or `"replace"`. Max 500 entries, no
duplicate asset ids, future dates rejected. Applied atomically:

```json
{ "data": { "date": "2026-08-01", "month": "2026-08",
    "written": 1, "replaced": 1, "payments_written": 1,
    "skipped": [ { "asset_id": 7, "reason": "asset is closed" } ],
    "duplicates": [ { "asset_id": 1, "existing_id": 411, "action": "replaced" },
                    { "asset_id": 2, "existing_id": 412, "action": "skipped" } ] } }
```

### `GET/PATCH/DELETE /api/v1/logs/{id}`

`PATCH` accepts any subset of `asset_id`, `date`, `deposit`, `value`:

```json
{ "value": "21410.00" }
```

### `GET /api/v1/logs/latest`

One row per asset - its most recent entry (id/order omitted when the asset
has none). Used by the quick grid and import previews:

```json
{ "data": [
    { "asset_id": 1, "description": "Bank savings", "closed": false,
      "id": 411, "at": "2026-08-31T00:00:00Z", "deposit": "24000.00", "value": "24012.80" },
    { "asset_id": 7, "description": "Apple Inc.", "closed": true }
  ] }
```

## Payments

Income events (dividends, interest, coupons) received *out* of an asset.

### `GET /api/v1/payments`

Same filters as logs (`asset_id`, `class_id`, `from`, `to`, `q`/`regex`,
`sort`, `page`, `per_page`):

```json
{ "data": [
    { "id": 96, "asset_id": 2, "at": "2026-08-15T00:00:00Z", "amount": "41.20" }
  ],
  "meta": { "page": 1, "per_page": 50, "total": 96 } }
```

### `POST /api/v1/payments`

```json
{ "asset_id": 2, "date": "2026-08-15", "amount": "41.20" }
```

### `GET/PATCH/DELETE /api/v1/payments/{id}`

`PATCH` accepts any subset of `asset_id`, `date`, `amount`.

## Settings

The settings table is effectively a **key-value store**: one row per
setting, a `setting` key (up to 20 characters) and a `value` string (up to
2048 characters). Each row also carries a `description` (up to 120
characters, the human label on the manage page) and a `display` flag
(opting the row in to that page). Nothing enforces a schema - the app
reads the few keys below and ignores the rest, so extra rows are harmless.

`db_version` and `db_migration_state` are managed by the migration runner
and refused on every write endpoint. Other keys are open, including
secret-convention keys (`*password*`, `*secret*`, `*token*`,
`*basicauth*`), whose values are redacted in every response.

### `GET /api/v1/settings`

Every stored row, one query:

```json
{ "data": [
    { "key": "club_note", "value": "hello", "description": "Club note", "display": true },
    { "key": "db_version", "value": "6", "description": "", "display": false },
    { "key": "projection_targets", "value": "[100000,250000]", "description": "", "display": false }
  ] }
```

Only stored rows are listed - there are no built-in defaults in the
payload (`projection_targets` stays absent until it is set, e.g. from the
Manage page's projections card).

### `POST /api/v1/settings/{key}`

Creates a row for a fresh key; the body seeds any combination of `value`,
`description` and `display` (defaults: empty and off):

```json
{ "value": "hello", "description": "Club note", "display": true }
```

```json
{ "data": { "key": "club_note", "value": "hello", "description": "Club note", "display": true } }
```

Creating a key that already exists is a `409 conflict` - patch it instead.
The response echoes the row (redacted for secret-convention keys).

### `PATCH /api/v1/settings/{key}`

Updates an existing row; the body is any combination of `value`,
`description` and `display`, and fields not sent are left alone. An
unknown key is a `404` (`POST` creates rows):

```json
{ "value": "[100000,250000]", "display": true }
```

The response echoes the full row as it now stands. `read_only` is not a
database setting (`CONSPECTUS_READ_ONLY` owns it), so patching it just
stores an inert value.

### `DELETE /api/v1/settings/{key}`

Deletes the setting row outright - value, description and display flag
all go, and subsequent reads fall back to the key's built-in default, or
to nothing for keys without one. To hide a key from the manage page
without deleting it, patch `display: false` instead.

```json
{ "data": { "deleted": "old_note" } }
```

All mutating endpoints are refused entirely in read-only mode
(`CONSPECTUS_READ_ONLY=true`), like every other data change.

## Statistics

All stats endpoints are served by the data API half and exist whenever
storage is wired - in frontend-only deployments (`CONSPECTUS_API_URL`)
they live on the remote API server instead.

All stats endpoints take the same scope/range query parameters:

| Param | Values |
|---|---|
| `asset_id` | scope to one asset |
| `class_id` | scope to one class |
| `from`, `to` | `YYYY-MM` range bounds |
| `granularity` | `month` (default) \| `quarter` \| `year` - re-buckets chart series |
| `closed` | `merged` (default: open individually + one aggregated closed series) \| `active` \| `all` |

Money values in stats payloads are major-unit numbers (chart-oriented);
months are `YYYY-MM` strings.

### `GET /api/v1/stats/overview`

KPI cards + sparkline + 1m/1y deltas:

```json
{ "data": {
    "months": 98, "assets": 12, "closed": 3,
    "value": 184219.15, "deposits": 151000.0, "return": 33219.15,
    "return_pct": 0.2200, "twrr": 0.969, "twrr_adj": 1.0236, "cagr": 0.0874,
    "sparkline": { "months": ["2025-06","2025-07","2025-08"],
                   "value": [171000.0, 176500.0, 184219.15],
                   "deposits": [148000.0, 149500.0, 151000.0] },
    "delta_1m": { "value": 7719.15, "deposits": 1500.0, "return": 6219.15,
                  "return_pct": 0.0414, "twrr": 0.0312, "twrr_adj": 0.0301 },
    "delta_1y": null,
    "notes": null } }
```

`twrr` is the time-weighted return of the whole chain (the growth index − 1);
`twrr_adj` additionally credits received payments back into the value
series. `notes` carries data-quality remarks (guarded chain legs).

### `GET /api/v1/stats/assets/{id}`

Overview scoped to one asset (same shape).

### `GET /api/v1/stats/series?metric=…`

Metrics: `value`, `deposits`, `return`, `return_pct`, `twrr`, `twrr_adj`,
`growth_index`, `decomposition`, `payments`.

```json
{ "data": { "months": ["2025-06","2025-07","2025-08"],
            "data": [171000.0, 176500.0, 184219.15], "notes": null } }
```

`decomposition` returns `new_money`/`growth` (+ cumulative variants). The
opening month of an unbounded range is omitted: with no prior month there is
no month-over-month change to split, only the initial deposit seed.

### `GET /api/v1/stats/breakdown`

Allocation tables, concentration and over-time grids:

```json
{ "data": {
    "classes": [ { "class_id": 1, "description": "Cash", "value": 24012.8,
                   "deposits": 24000.0, "return": 12.8, "return_pct": 0.0005,
                   "twrr_adj": 0.0005, "share": 0.1304, "legs": 98,
                   "short_history": false } ],
    "assets": [ { "asset_id": 2, "class_id": 2, "description": "Vanguard ISA",
                  "value": 21406.35, "deposits": 18250.0, "return": 3156.35,
                  "return_pct": 0.1728, "twrr_adj": 0.1888, "share": 0.1162,
                  "legs": 98, "short_history": false } ],
    "concentration": { "largest_share": 0.4102, "largest_asset": "Workplace pension",
                       "hhi": 0.2314, "effective_n": 4.3 },
    "closed": { "count": 3, "total_deposits": 21000.0, "total_value": 22500.0,
                "realized_gain": 1500.0,
                "assets": [ { "asset_id": 7, "description": "Apple Inc.", "class_id": 2,
                              "deposits": 1500.0, "value": 1875.0, "gain": 375.0,
                              "last_month": "2024-11" } ] },
    "allocation_over_time": { "months": ["2025-06","2025-07"],
                              "series": { "Cash": [0.141, 0.136], "Shares": [0.859, 0.864] } },
    "assets_over_time": { "months": ["2025-06","2025-07"],
                          "series": { "Bank savings": [0.141, 0.136] } } } }
```

### `GET /api/v1/stats/analytics`

```json
{ "data": {
    "yearly": [ { "year": 2026, "start": 176500.0, "end": 184219.15,
                  "change_pct": 0.0437, "deposits": 1500.0, "growth": 5219.15,
                  "income": 41.2, "twrr": 0.0312, "twrr_adj": 0.0301 },
                { "year": 2025, "start": 120000.0, "end": 176500.0,
                  "change_pct": 0.4708, "deposits": 28000.0, "growth": 27500.0,
                  "income": 0.0, "twrr": 0.1984, "twrr_adj": 0.1984 } ],
    "records": { "up_streak": 7, "down_streak": 3,
                 "best_month": { "month": "2025-03", "return": 0.0612 },
                 "worst_month": { "month": "2024-10", "return": -0.0471 },
                 "max_drawdown_growth": { "depth": 0.1320, "peak": "2021-11", "trough": "2022-09" },
                 "volatility_monthly": 0.0241, "volatility_annual": 0.0835,
                 "return_per_risk": 1.0467, "positive_share": 0.6327,
                 "monthly_mean": 0.0068, "monthly_median": 0.0071 },
    "histogram": { "bins": ["-5.00%..-4.00%", "-4.00%..-3.00%"],
                   "counts": [1, 2] },
    "heatmap": { "years": [2024, 2025],
                 "cells": { "2024": [0.021, null, null, null, null, null, null, null,
                                     null, -0.047, 0.011, 0.018] } },
    "decomposition": { "months": ["2025-07","2025-08"],
                       "new_money": [1500.0, 1500.0], "growth": [5000.0, 6219.15],
                       "cum_new_money": [1500.0, 3000.0], "cum_growth": [5000.0, 11219.15] } } }
```

(`yearly.start` is the opening balance carried into the year - the prior
year-end close, ≈0 at inception - so `start + deposits + growth − income =
end` holds exactly; `change_pct` is `null` for the inception year.)

### `GET /api/v1/stats/projections`

Extra params: `horizon` (months, default 120, max 600), `window` (trailing
return window in months, default all history).

```json
{ "data": {
    "horizon_months": 120, "from_month": "2025-08",
    "months": ["2025-09", "2025-10"],
    "expected": [184900.0, 185800.0],
    "upper": [192000.0, 194500.0],
    "lower": [177800.0, 176900.0],
    "cagr_path": [185700.0, 187200.0],
    "milestones": [ { "target": 250000.0, "months": 94, "date": "2033-06",
                      "reached": true, "already": false } ],
    "assumptions": { "value_now": 184219.15, "mu": 0.0068, "sigma": 0.0241,
                     "deposit_monthly": 1250.0, "window": "all" },
    "target_curve": { "value": 1000000.0, "year": 2048, "months_remaining": 276,
                      "required_monthly": 0.0049, "required_cagr": 0.0610,
                      "curve_months": ["2026", "2027"],
                      "curve_values": [200100.0, 218300.0] },
    "caveat": "Extrapolations of past behaviour - not promises." } }
```

On an empty database the payload is empty-but-valid (`months`/`expected`
null, `from_month` `""`).

### `GET /api/v1/stats/income`

```json
{ "data": {
    "total": 3814.55, "zero_months": 41,
    "by_month": [ { "month": "2026-08", "amount": 44.40 } ],
    "by_year": [ { "year": 2026, "amount": 512.75 } ],
    "by_class": [ { "class_id": 2, "description": "Shares", "amount": 512.75 } ],
    "top_assets": [ { "asset_id": 2, "description": "Vanguard ISA", "amount": 288.15 } ],
    "ttm_yield": 0.0028, "yoy_growth": 0.1120,
    "idle_cash": [ { "asset_id": 4, "description": "Old current account",
                     "value": 5000.0, "twrr_12m": 0.0001 } ] } }
```

### `GET /api/v1/stats/quality`

Whole-dataset data-quality scan (ignores scope):

```json
{ "data": {
    "duplicate_months": [ { "asset_id": 2, "month": "2023-07", "count": 2 } ],
    "gaps": [ { "asset_id": 3, "first_month": "2022-01", "last_month": "2022-06", "months": 5 } ],
    "stale_assets": [ { "asset_id": 4, "description": "Old current account",
                        "last_month": "2024-02", "months_since": 18 } ],
    "future_rows": [] } }
```

## UI row feeds

JSON feeds backing the client-side table pagers (filters ride the browser's
UI cookie). Page number and page size are path segments. These are served
by the web frontend half, so they exist only when the UI is enabled - in
API-only mode (`CONSPECTUS_WEB=false`) they 404, and in a frontend-only
split (`CONSPECTUS_API_URL`) the frontend process answers them locally:

- `GET /api/ui/v1/logs/p/{n}[/per/{per}]`
- `GET /api/ui/v1/payments/p/{n}[/per/{per}]`
- `GET /api/ui/v1/assets/{id}/payments/p/{n}[/per/{per}]`
- `GET /api/ui/v1/classes/{id}/payments/p/{n}[/per/{per}]`

`{per}` is one of `10, 20, 50, 100, 200, all` (default 20):

```json
{ "data": { "rows": [
      { "id": 411, "date": "2026-08-31", "asset": "Bank savings",
        "deposit": "24000.00", "value": "24012.80" }
    ], "page": 1, "pages": 9, "total": 412 } }
```
