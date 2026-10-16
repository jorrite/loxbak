# loxbak architecture

This document describes how loxbak is put together. It follows the code as
it is now; for setup and usage, see [README.md](./README.md).

## What it does

loxbak makes scheduled backups of a Loxone Miniserver. The Miniserver has
no "download a backup" endpoint, so loxbak logs in to its FTP(S) service,
mirrors the files into a local directory, zips that directory, and hands
the zip to one or more configured **destinations** (local disk, WebDAV, or
a remote FTP server). Each **schedule** has a cron expression, a list of
destinations, and a retention count for each destination. A small web GUI
manages all of this.

All of it ships as **one Go binary**: HTTP API, cron scheduler, SQLite
database and the statically exported Next.js frontend in one process. It
needs no other services.

```
                 ┌──────────────────────── loxbak process ────────────────────────┐
 browser ──HTTP──▶ api (net/http mux) ──▶ web (embedded Next.js static export)    │
                 │   │  session cookie auth                                      │
                 │   ├──▶ db (SQLite, modernc.org/sqlite) ◀──┐                   │
                 │   │                                       │                   │
                 │   └──▶ scheduler ──(robfig/cron)── RunSchedule / TriggerRun   │
                 │                      │                                        │
                 │                      ├──▶ ftp.Mirror ──FTP(S)──▶ Miniserver   │
                 │                      ├──▶ zip archive ($TMPDIR)               │
                 │                      └──▶ destinations ──▶ local / WebDAV / FTP│
                 └───────────────────────────────────────────────────────────────┘
                        $DATA_DIR: loxbak.db, mirror/, backups/<dest-name>/
```

## Repository layout

| Path | What lives there |
|------|------------------|
| `cmd/loxbak/main.go` | Entry point: reads env, opens DB, starts scheduler and HTTP server, handles shutdown. |
| `internal/api/` | HTTP handlers, routing, CORS/logging/session middleware. |
| `internal/scheduler/` | Cron wrapper (`scheduler.go`) and the backup job pipeline (`job.go`). |
| `internal/ftp/` | Miniserver FTP client: credential check and incremental mirror. |
| `internal/destinations/` | `Destination` interface, type registry, retention pruning, and the `local`, `webdav`, `ftp` implementations. |
| `internal/db/` | SQLite connection, embedded SQL migrations, and one file of query functions per table. |
| `internal/session/` | HMAC-signed session cookies backed by the `sessions` table. |
| `internal/crypto/` | AES-256-GCM encryption of secrets at rest. |
| `internal/web/` | `go:embed` of the frontend build and its `http.Handler`. `static/index.html` is a committed placeholder that the build replaces. |
| `web/` | Next.js (App Router) frontend, built with bun as a static export. |
| `justfile` | Dev, build, check and test recipes. CI calls these too. |
| `Dockerfile` | Three-stage build: bun frontend → Go binary (CGO off) → distroless image. |
| `.github/workflows/` | `ci.yml` (checks and tests on PRs) and `release.yml` (release-please, binaries, multi-arch GHCR image). |

## Backend components

### Startup (`cmd/loxbak`)

`run()` in `cmd/loxbak/main.go`:

1. Reads `MASTER_KEY` (required), `PORT` (default `8080`) and `DATA_DIR`
   (default `/data`), and creates `DATA_DIR`.
2. `db.Open("$DATA_DIR/loxbak.db")` opens SQLite and runs migrations.
3. `ensureSessionSecret` loads the session-signing secret from the
   `settings` table, or generates and stores one on first run. It is not an
   env var.
4. `scheduler.New` → `Reload()` (loads enabled schedules into cron) → `Start()`.
5. `api.New(...).ListenAndServe()` runs in a goroutine.
6. On SIGINT/SIGTERM: `sched.Stop` waits for cron to stop, but runs already
   in progress are not interrupted. Then `server.Shutdown` runs, with a 10s
   timeout for both.

The `*sql.DB`, `MASTER_KEY` and the session key are passed in explicitly.
There is no global state apart from the destination registry and the
mirror mutex (see below).

### HTTP API (`internal/api`)

- `server.go`: `Server` holds the DB, master key, session key and
  scheduler. The handler chain is `withLogging(withCORS(Router()))`.
- `router.go`: Go 1.22 pattern routing. Every `/api/*` route except
  `health`, `login` and `logout` is wrapped in `requireSession`. The
  catch-all `/` serves the embedded frontend.
- `middleware.go`: `withCORS` reflects the request origin only for
  `localhost`/`127.0.0.1` (for the dev setup with the frontend on :3000).
  `requireSession` validates the cookie and puts the session in the
  request context.

| Routes | Handler file | Notes |
|--------|--------------|-------|
| `GET /api/health` | `health.go` | Unauthenticated. |
| `POST /api/login`, `POST /api/logout`, `GET /api/me` | `auth.go` | See [Authentication](#authentication). |
| `/api/schedules[/{id}]` CRUD, `POST /api/schedules/{id}/run` | `schedules.go` | Every mutation calls `Scheduler.Reload()`. `run` returns `202` with a `running` run row. |
| `/api/destinations[/{id}]` CRUD, `POST …/{id}/test` | `destinations.go` | Secrets are encrypted on write and never returned (`has_secret` only). Delete gives `409` if a schedule still uses the destination; `?purge=true` also deletes the stored archives. `test` returns `501` (not implemented yet). |
| `GET /api/runs`, `GET /api/runs/{id}` | `runs.go` | Run history from the DB, with per-destination results. |
| `GET /api/backups`, `GET …/{destinationId}/{filename}/download`, `DELETE …/{destinationId}/{filename}` | `backups.go` | Lists what is actually stored on each destination, not the DB history. See [Backups view](#backups-view-reality-over-history). |

Errors are JSON `{"error": "..."}` (`writeError`).

### Scheduler and backup job (`internal/scheduler`)

- `scheduler.go`: wraps `robfig/cron/v3`. `Reload()` removes all cron
  entries and adds one per enabled schedule, keyed by schedule id. An
  invalid cron expression is logged and that schedule is skipped.
- `job.go`: the backup pipeline. It has two entry points that share
  `executeAndFinish` → `doRun`:
  - `RunSchedule`: the cron path. Synchronous, 2h timeout.
  - `TriggerRun`: the "run now" path. It creates the `runs` row, returns
    its id straight away and runs the backup in a goroutine. The UI polls
    `GET /api/runs`.

### Miniserver FTP client (`internal/ftp`)

- `Connect` tries explicit TLS (FTPES, `InsecureSkipVerify` because
  Miniservers use self-signed or LAN-only certificates) and falls back to
  plain FTP. The `ftp` destination uses it too.
- `Validate` does a real login. It is used by `POST /api/login`, and
  connection errors get a hint that FTP is disabled by default since
  firmware 16.1.
- `Mirror` syncs the remote tree like rsync into a persistent local
  directory:
  - A hand-written recursive walk replaces `goftp.Walker`, so a directory
    that cannot be listed only skips that subtree.
  - Files with the same size and an mtime within 2 minutes count as
    unchanged and are not downloaded again. Downloaded files get the
    remote mtime.
  - Local files that are gone on the Miniserver are deleted.
  - Each listing and download is retried 5 times with exponential backoff
    (500ms…8s), then skipped with a warning. One unreadable file does not
    fail the backup.
  - `/sys` (firmware images) is excluded.

### Destinations (`internal/destinations`)

`destination.go` defines a small plugin model:

- `Destination` (required): `Name()`, `Store(ctx, BackupMeta, io.Reader)`.
- `Lister` (optional): `List`, `Delete`. Retention pruning, the Backups
  page and purge-on-delete need it.
- `Reader` (optional): `Open`. The Backups page download needs it.
- A registry: each implementation calls `Register(type, constructor)` in
  `init()`. `Build(type, name, configJSON, secret)` constructs one from a
  DB row. `config_json` is per-type JSON. The secret (password) is stored
  encrypted in a separate column and passed in already decrypted.
- `Prune(ctx, lister, prefix, keep)`: shared retention logic. It filters
  by the schedule's filename prefix, sorts newest first (the timestamp in
  the filename sorts lexically) and deletes everything after `keep`.

| Type | File | Stores to | Lister | Reader |
|------|------|-----------|:------:|:------:|
| `local` | `local.go` | `$DATA_DIR/backups/<destination name>/` | ✓ | ✓ |
| `webdav` | `webdav.go` | `<url>/<dir>/` via `studio-b12/gowebdav` | ✓ | — |
| `ftp` | `ftp.go` | `<host>:<port>/<dir>/` via `internal/ftp.Connect` + STOR | ✓ | — |

Adding a type means a new file with `init()` registration plus a config
form in `web/app/destinations/page.tsx`. Migration `0002` removed the
`type` CHECK constraint so that no schema change is needed.

### Persistence (`internal/db`)

- SQLite through pure-Go `modernc.org/sqlite`, which is why `CGO_ENABLED=0`
  builds and the distroless image work. Settings: one open connection
  (`SetMaxOpenConns(1)`), WAL, `busy_timeout(5000)`, foreign keys on.
- Migrations: `migrations/*.sql` are embedded and applied in filename
  order. Files whose numeric prefix is at or below `settings.schema_version`
  are skipped. `0001` is idempotent (`IF NOT EXISTS`). Later migrations
  (table rebuild, `ADD COLUMN`) rely on the version gate.
- Query functions are plain functions that take `*sql.DB`, one file per
  table. There is no ORM.

Tables (`0001_init.sql` plus later migrations):

| Table | Purpose |
|-------|---------|
| `settings` | Key/value: `schema_version`, `session_signing_secret`. |
| `sessions` | Session ids with expiry (30 days). |
| `loxone_credential` | Miniserver host, port, username, encrypted password. The schema allows several rows, but the app only uses the lowest id. |
| `destinations` | name, type, `config_json`, `encrypted_secret`. |
| `schedules` | name, `cron_expr`, enabled. |
| `schedule_destinations` | schedule ↔ destination link with `retention_count` (0 = keep all). |
| `runs` | One row per execution: times, status, size, error, archive `filename` (added in `0003`). |
| `run_destination_results` | Per-destination outcome of a run. |

### Security pieces

- **`internal/crypto`**: AES-256-GCM, key = SHA-256(`MASTER_KEY`). The
  output is `nonce‖ciphertext`. It protects the Miniserver password and
  destination secrets. If `MASTER_KEY` changes, the stored secrets can no
  longer be decrypted.
- **`internal/session`**: the cookie `loxbak_session` = `id.HMAC-SHA256(id)`.
  It is HttpOnly, SameSite=Lax and valid for 30 days. On validation the
  HMAC is checked first, then the `sessions` row and its expiry.

### Embedded frontend (`internal/web`)

`//go:embed all:static`. The `all:` prefix is needed so that `_next/`
is included. It is served with `http.FileServerFS`. The repo only holds a
placeholder `index.html`. `just build-backend` and the Dockerfile copy
`web/out/` into `internal/web/static/` before `go build`.

## Main flows

### Authentication

loxbak has no user accounts of its own: **the Miniserver credentials are
the login.**

1. `POST /api/login {host, port, username, password}` → `ftp.Validate`
   does a live FTP login against the Miniserver.
2. If that works, the password is encrypted and upserted into
   `loxone_credential`. This is the credential the scheduler uses later.
3. `session.Create` inserts a `sessions` row and sets the signed cookie.
4. The frontend's `NavShell` calls `GET /api/me` on every page load and
   sends you to `/login/` on a 401.

### A backup run (`scheduler.doRun`)

```
cron tick ─▶ RunSchedule ─┐
"Run now" ─▶ TriggerRun ──┴─▶ runs row (status=running)
                               │
  load credential (decrypt) ◀──┤
  load schedule + destination links (fail if none)
                               │
  ┌── mirrorMu (one at a time) ┤
  │  ftp.Mirror → $DATA_DIR/mirror/
  │  archiveDir → $TMPDIR/<SanitizeName(schedule)>-<20060102T150405Z>.zip
  └────────────────────────────┤
  for each destination link:   │
     Build destination (decrypt secret) → Store(zip)
     if retention_count > 0 and it is a Lister → Prune by schedule prefix
        └─ delete matching run_destination_results rows
     insert run_destination_results(success|failed)
                               │
  FinishRun(status, size, filename, error); remove the temp zip
```

- `mirrorMu` serializes only the mirror and zip step, because every run
  shares the one persistent `mirror/` directory. Uploads to destinations
  run without the lock.
- Run status: `success` if all destinations succeed. Otherwise `failed`,
  with an error message that says whether it was a partial or a total
  failure. The schema and the frontend types also allow `partial`, but
  the job code never writes it now.
- A failed prune is only logged and does not fail the run.
- The archive filename (`<schedule>-<UTC timestamp>.zip`) carries meaning:
  retention pruning matches on its prefix, and the Backups view parses
  schedule and time out of it (`SanitizeName`, `ParseArchiveTimestamp`).

### Backups view: reality over history

`GET /api/backups` builds every destination and calls `List()` on each
one that is a `Lister`. It then adds data to each file in this order:

1. If a matching `runs`/`run_destination_results` row exists (joined on
   `runs.filename`), the schedule and start/finish times come from there.
2. If not, the schedule is matched by filename prefix and the start time
   is parsed from the filename. If that fails, the file mtime is used.

This means archives from before loxbak tracked runs, or archives uploaded
or deleted outside loxbak, show up correctly. Download is only offered
when the destination implements `Reader`. Delete goes through `Lister`
and removes the matching result row.

### Config changes

Schedule create, update and delete (and destination delete) write to
SQLite and then call `Scheduler.Reload()`, which rebuilds all cron
entries from `schedules WHERE enabled = 1`. Destination links are replaced
as a whole by `SetScheduleDestinations`.

## Frontend (`web/`)

- Next.js 16 App Router, React 19, Tailwind CSS 4, Radix UI primitives,
  Geist fonts. `next.config.ts` sets `output: "export"` and
  `trailingSlash: true`, so it builds to plain static files with no Node
  server at runtime.
- Pages: `/` (dashboard: schedules, recent runs, run-now, polls while a
  run is `running`), `/destinations/`, `/schedules/`, `/backups/`,
  `/login/`. All of them are client components that fetch data at runtime.
- `lib/api.ts` is the only API client: a typed `fetch` wrapper with
  `credentials: "include"`. In production it uses relative paths (same
  origin). Under `next dev` it calls `http://localhost:8080`, which can be
  overridden with `NEXT_PUBLIC_API_BASE`.
- `components/nav/NavShell.tsx` holds the layout and the auth gate.
  `components/ui/` has shared UI pieces, `components/ascii/` has the
  terminal-style boot and spinner effects.

## Configuration and runtime data

| Env var | Default | Used by |
|---------|---------|---------|
| `MASTER_KEY` | — (required) | `crypto` key derivation |
| `PORT` | `8080` | HTTP listen address |
| `DATA_DIR` | `/data` | `loxbak.db`, `mirror/`, `backups/<name>/` (read in `main.go`, `scheduler/job.go`, `destinations/local.go`) |

`$DATA_DIR` must be persistent: it holds the database and the incremental
mirror, and also the archives of `local` destinations. Temporary zips go
to `os.TempDir()` and are deleted after each run.

## Key dependencies

| Dependency | Role |
|------------|------|
| `github.com/jlaffaye/ftp` | FTP/FTPES client (Miniserver source and `ftp` destination) |
| `github.com/robfig/cron/v3` | Cron scheduling |
| `github.com/studio-b12/gowebdav` | WebDAV destination |
| `modernc.org/sqlite` | Pure-Go SQLite driver (no CGO) |
| Go standard library | `net/http` routing, `archive/zip`, `crypto/*`, `log/slog`, `embed` |
| Next.js, React, Tailwind, Radix UI | Frontend (`web/package.json`) |

## Build, check and release

- `just dev`: `go run ./cmd/loxbak` (with `DATA_DIR=.data`) and `next dev`
  side by side. The CORS middleware allows the cross-origin dev setup.
- `just build`: frontend static export → copied into `internal/web/static`
  → `go build` → `bin/loxbak`. The placeholder is restored afterwards.
- `just check` / `just test`: gofmt and go vet, tsc and eslint, go build
  and go test, next build. CI (`ci.yml`) runs the `-backend` and
  `-frontend` variants as separate jobs on pull requests.
- `release.yml`: release-please on `main`. When a release is cut, it
  publishes linux amd64/arm64 tarballs and a multi-arch image to
  `ghcr.io`.
