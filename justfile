set dotenv-load := true

default:
    @just --list

# Run the backend (go run) and the frontend dev server (next dev,
# in web/) concurrently for local dev.
dev:
    #!/usr/bin/env bash
    set -uo pipefail
    if [ ! -f .env ]; then
        echo "No .env found. Run: cp .env.example .env  (then edit MASTER_KEY if you want a non-default value)" >&2
        exit 1
    fi

    backend_port="${PORT:-8080}"
    frontend_port="${FRONTEND_PORT:-3000}"
    # Catch the "previous `just dev` didn't shut down cleanly" case (e.g. the
    # terminal was closed instead of Ctrl-C'd, orphaning its children) before
    # it turns into a confusing bind-error cascade from the backend itself.
    if command -v lsof >/dev/null 2>&1; then
        for p in "$backend_port" "$frontend_port"; do
            pid="$(lsof -tiTCP:"$p" -sTCP:LISTEN 2>/dev/null || true)"
            if [ -n "$pid" ]; then
                echo "Port $p is already in use (pid $pid) - probably a stale process from a previous 'just dev' that didn't exit cleanly." >&2
                echo "Run 'just dev-stop' to clear it, then try again." >&2
                exit 1
            fi
        done
    fi

    trap 'kill 0 2>/dev/null' EXIT
    trap 'exit 130' INT
    trap 'exit 143' TERM
    (DATA_DIR="${DATA_DIR:-.data}" PORT="$backend_port" go run ./cmd/loxbak) &
    backend_pid=$!
    # Next dev also honors a PORT env var - set it explicitly to
    # frontend_port rather than letting it fall through from whatever PORT
    # happens to be set in the ambient shell (which would otherwise collide
    # with the backend's own PORT).
    (cd web && PORT="$frontend_port" bun run dev) &
    frontend_pid=$!
    # No `wait -n` here: macOS ships bash 3.2 (wait -n needs bash >=4.3), so
    # poll instead. Whichever side dies first, kill the other and propagate
    # its exit code, rather than hanging until the survivor is Ctrl-C'd.
    while kill -0 "$backend_pid" 2>/dev/null && kill -0 "$frontend_pid" 2>/dev/null; do
        sleep 1
    done
    if kill -0 "$backend_pid" 2>/dev/null; then
        wait "$frontend_pid"; exit $?
    else
        wait "$backend_pid"; exit $?
    fi

# Kill anything left listening on the dev ports from a `just dev` that didn't
# exit cleanly (e.g. the terminal was closed rather than Ctrl-C'd).
dev-stop:
    #!/usr/bin/env bash
    set -uo pipefail
    if ! command -v lsof >/dev/null 2>&1; then
        echo "lsof not found - can't detect stale processes automatically." >&2
        exit 1
    fi
    found=0
    for p in "${PORT:-8080}" "${FRONTEND_PORT:-3000}"; do
        pid="$(lsof -tiTCP:"$p" -sTCP:LISTEN 2>/dev/null || true)"
        if [ -n "$pid" ]; then
            echo "killing pid $pid on port $p"
            kill "$pid"
            found=1
        fi
    done
    if [ "$found" = 0 ]; then
        echo "nothing found on ports ${PORT:-8080}/${FRONTEND_PORT:-3000}"
    fi

# Build the frontend static export only.
build-frontend:
    cd web && bun install --frozen-lockfile && bun run build

# Embed the frontend static export into the backend and build the binary.
# Cross-compiles via the ambient GOOS/GOARCH/CGO_ENABLED env vars, same as
# any `go build` — set them before calling this for a non-host target
# (see release.yml, which builds this once per release platform).
build-backend: build-frontend
    #!/usr/bin/env bash
    set -euo pipefail
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT
    cp -a internal/web/static/. "$tmp/"
    rm -rf internal/web/static/*
    cp -a web/out/. internal/web/static/
    go build -trimpath -ldflags="-s -w" -o bin/loxbak ./cmd/loxbak
    rm -rf internal/web/static/*
    cp -a "$tmp/." internal/web/static/

build: build-backend

# Multi-arch Docker image build.
docker-build:
    docker buildx build --platform linux/amd64,linux/arm64 -t loxbak:local .

# `./cmd/... ./internal/...`, not the bare `./...` wildcard: run from repo
# root now that the module lives there (see Repo layout), `./...` also
# recurses into web/node_modules looking for Go packages — and finds one
# (an npm package that happens to bundle a Go implementation alongside its
# JS), so an unscoped `./...` can pick up and fail on code that has
# nothing to do with this module.
fmt:
    gofmt -l -w cmd internal && go vet ./cmd/... ./internal/...
    cd web && bun run lint --fix

# Read-only static checks — fails rather than fixing, unlike `fmt`. Split
# into -backend/-frontend so CI can run each in its own (Go-only /
# bun-only) job; `just check` runs both for local convenience. This is
# what CI actually calls, via `just check`/`just test`, rather than CI
# keeping its own separate copy of these commands.
check-backend:
    test -z "$(gofmt -l cmd internal)"
    go vet ./cmd/... ./internal/...

check-frontend:
    cd web && bunx tsc --noEmit
    cd web && bun run lint

check: check-backend check-frontend

test-backend:
    go build ./cmd/... ./internal/...
    go test ./cmd/... ./internal/...

test-frontend:
    cd web && bun run build

test: test-backend test-frontend
