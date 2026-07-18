<p align="center">
  <img src="web/app/icon.svg" width="72" height="72" alt="loxbak logo" />
</p>

<h1 align="center">loxbak</h1>

<p align="center">
  Scheduled, pluggable backups for a Loxone Miniserver — one Docker
  container, one small web GUI, no cloud dependency.
</p>

> **Unofficial.** Not affiliated with, endorsed by, or supported by Loxone.
> "Loxone" and "Miniserver" are trademarks of their respective owner; this
> project just talks to the device over the FTP interface it already
> exposes.

## What this is

A Loxone Miniserver has no single-file "give me a backup" endpoint — it
exposes project/config data over FTP (the same admin credentials as the
web UI or Loxone Config), and firmware ≥ 16.1 disables that FTP access by
default, so you'll need to turn it on in the Miniserver's own settings
first. loxbak connects to that FTP server, mirrors the relevant files
incrementally (only re-downloading what changed since the last run),
zips the result, and hands the archive to one or more destinations you
configure — on a cron schedule, or on demand.

Everything runs as a single Go binary with an embedded web UI: no
separate database server, no message queue, nothing else to deploy.

## Features

- **Multiple schedules**, each with its own cron expression and
  destinations — e.g. daily to local storage, weekly to WebDAV, with
  independent retention per destination.
- **Three destination types**: local disk, WebDAV, and remote FTP(S),
  each pluggable — adding a new one is a small, self-contained change.
- **Retention/pruning**: keep the newest N backups per schedule per
  destination and let loxbak delete the rest automatically.
- **A backups view that reflects reality**: it lists what's actually
  stored on each destination, not just what loxbak's own history
  remembers, so it also surfaces (and can remove) backups that predate
  loxbak tracking them at all.
- **No separate account system.** The Miniserver credentials you log in
  with are validated live against the device itself and are what the
  scheduler later uses to run backups — nothing extra to manage.
- Credentials and destination secrets are encrypted at rest.

## Prerequisites

- `go`, `bun`, and `just` on your PATH (e.g. via [mise](https://mise.jdx.dev/))
- Docker (with buildx) for building the container image

## Development

```sh
cp .env.example .env   # set MASTER_KEY
just dev                # runs backend (go run) + frontend (next dev) concurrently
```

- Frontend dev server: http://localhost:3000
- Backend API/health: http://localhost:8080/api/health

## Building

```sh
just build          # frontend static export -> embedded into the Go binary at bin/loxbak
just docker-build    # multi-arch (amd64/arm64) Docker image
```

## Runtime configuration

| Env var      | Required | Default | Purpose                                             |
|--------------|----------|---------|------------------------------------------------------|
| `MASTER_KEY` | yes      | —       | Key used to encrypt stored credentials at rest       |
| `PORT`       | no       | `8080`  | HTTP port                                            |
| `DATA_DIR`   | no       | `/data` | Where the SQLite DB and local-destination backups live |

Mount `/data` (or your `DATA_DIR`) as a persistent volume — it holds the
SQLite database and any `local`-destination backup archives.

## License

[MIT](./LICENSE).

If you end up running loxbak at scale or as part of something commercial,
I'd genuinely like to hear about it — open an issue or drop a line. Not a
requirement, just a nice thing to know.
