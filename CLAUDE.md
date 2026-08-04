# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

DokVol is a Docker Volume Manager appliance: it scans Docker volumes/bind mounts, maps them to physical drives, tracks disk usage over time, migrates volume data between drives (via rsync), and backs up volumes to remote targets (S3, SFTP, local). It ships as a single Docker image containing a Go backend and a SvelteKit SPA, both served from the same process on port 8080.

## Repository layout

- `api/` — Go backend (module `dokvol/api`), Gin HTTP server + CLI (Cobra)
- `interface/` — SvelteKit 5 frontend (static adapter), built and served by the Go backend
- `Dockerfile` — multi-stage build (bun frontend → golang backend → alpine runtime)
- `Dockerfile.test` / `docker-compose.test.yml` — privileged container used to run the E2E test suite
- `.gitlab-ci.yml` — CI: test (privileged container) → build → publish to `ghcr.io/thirdshop-org/dokvol` → bump `VERSION` on tag

## Common commands

### Backend (`api/`)

```sh
cd api
go run ./cmd/server/          # simple dev entrypoint
go run ./cmd/dokvol/ server   # Cobra CLI entrypoint (same server, `dokvol server`)
go build ./...
go vet ./...

go test ./...                              # unit tests (system/, internal/handler/, etc.)
go test ./system/... -run TestName -v       # single test
```

E2E tests (`api/system/e2e/`) require Docker-in-Docker, loop devices, and privileged access — they only run reliably inside `Dockerfile.test`:

```sh
DOCKER_BUILDKIT=0 docker build -f Dockerfile.test -t sut-test .
docker run --privileged -v /dev:/dev -e DOKVOL_DB_PATH=/tmp/dokvol-test.db sut-test
```

That container's default CMD is `go test ./system/... -v -run TestE2E -count=1 -timeout=180s`.

Unit tests that touch the DB spin up a temp SQLite file and run goose migrations against `../migrations` (see `system/stats_test.go`, `internal/handler/stats_test.go`) or use `system/internal/testutil.InitTestDB` — no shared test DB/fixtures.

### Database / sqlc

Schema and hand-written queries live in `api/sql/schema.sql` and `api/sql/queries.sql`. Generated code goes to `api/internal/db/` (`sqlc.yaml` config). After editing `sql/*.sql`, regenerate with `sqlc generate` (run from `api/`) — do not hand-edit files in `internal/db/`.

Versioned migrations (goose) live in `api/migrations/NNNNN_*.sql` and run automatically at startup (`internal/database/sqlite.go`) and in tests. Add a new migration file rather than editing an applied one.

### Frontend (`interface/`)

```sh
cd interface
bun install
bun run dev      # :5173, proxies /api to localhost:8080 (see vite.config.ts)
bun run build    # static output to build/, embedded into the Docker image
bun run check    # svelte-kit sync + svelte-check
```

### Full stack via Docker

```sh
docker build -t dokvol:dev .
docker run -d --privileged \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /var/lib/docker/volumes:/var/lib/docker/volumes:rslave \
  -v /mnt:/mnt:rslave \
  -v /etc/dokvol:/etc/dokvol \
  -p 8080:8080 dokvol:dev
```

The backend needs the Docker socket and `--privileged` (mount/loop device access) even in dev, since it inspects containers/volumes and mounts drives directly — plain `go run` on a dev machine works for API/DB logic but drive/volume features will no-op or error without these.

## Architecture

### Backend structure (`api/`)

- `cmd/dokvol/` — Cobra CLI; `server.go` is the actual composition root: loads `.env` (search order `/etc/dokvol/.env`, `.env`, `$DOKVOL_ENV`), opens the DB, wires `handler.DB` / `handler.MigrationManager` / `handler.BackupEngine` as package-level globals, starts the backup scheduler and stats collector as background goroutines, registers all routes, and serves the built SvelteKit SPA as a catch-all (`r.NoRoute`) for anything not under `/api`.
- `cmd/dokvol/{apps,backup,drives,migrate}.go` — CLI subcommands for direct (non-server) operations, calling into the same `system`/`system/backup` packages as the HTTP handlers.
- `internal/db/` — sqlc-generated repository code (`Queries`, models). Never edit by hand; regenerate from `sql/`.
- `internal/database/sqlite.go` — opens the SQLite connection, runs goose migrations, triggers a drive-history rescan on boot.
- `internal/auth/` — JWT access/refresh token issuance & validation, password hashing.
- `internal/middleware/auth.go` — `AuthRequired()` populates `user_id`/`username`/`role` in the Gin context from the bearer token; `AdminRequired()` gates on `role == "admin"`. Auth errors use structured `error_code` values (e.g. `AUTH.TOKEN_EXPIRED`, `AUTH.UNAUTHORIZED`) that the frontend matches on.
- `internal/handler/` — one file per resource, thin Gin handlers that call into `system`/`system/backup` and `handler.DB`. `handler.MigrationManager` and `handler.BackupEngine` (set once in `server.go`) are the shared long-lived state for async job tracking.
- `system/` — core domain logic, independent of HTTP: Docker introspection (`docker.go`, `applications.go`), drive discovery/health (`drives.go`, `system.go`), volume migration (`migration.go`, `storage.go`), stats collection (`stats.go`), history (`history.go`), config encryption (`encryption.go`), and a structured `APIError` type (`errors.go`) with an `error_code`/`HTTPStatus()` mapping used across handlers.
- `system/backup/` — backup subsystem: `engine.go` runs backup jobs (rsync to a target), `target.go` defines provider configs (S3/SFTP/Local) whose secrets are encrypted at rest via `system.EncryptConfig`/`DecryptConfig`, `scheduler.go` implements a simple cron-expression matcher and retention pruning, `list.go`/`restore.go` handle listing and restoring existing backups.
- `system/e2e/` — real Docker + loop-device + rsync integration tests; see `system/internal/testutil` for the shared harness (spin up containers/volumes, create loopback ext4 drives, poll async jobs).

### Migration job model

Volume migrations and backups both follow the same async-job pattern: an HTTP handler starts a job, persists a row (`migration_jobs`/`backup_jobs` + per-volume progress rows) via sqlc queries, then launches a goroutine (`MigrationManager.runJob` / `BackupEngine.RunBackup`) that streams progress through an `OnProgress` callback. Progress is kept both in an in-memory `map[string]*Job` (for fast polling) and persisted to SQLite (so jobs survive restarts and can be listed via `ListJobsWithProgress`-style queries). When reading job state, `GetJob`/`ListJobs` check the in-memory map first and fall back to reconstructing from the DB.

A migration's actual data move (`migrateVolume` in `system/storage.go`) runs as a crash-safe pipeline: `LockVolume` (an flock on a hashed marker under `/etc/dokvol/locks`) makes it mutually exclusive with any other migration or backup touching the same source path; a live presync pass (`StepPresync`) copies the bulk of the data while the container is still running, so only a delta is left once it stops; `relink()` renames the original directory aside to a `.dokvol-bak` sibling (never deletes it outright) before symlinking, and only reclaims it once the container is confirmed healthy again. If the process dies mid-migration, `ReconcileMigrationJobs` (run once at boot, in `system/reconcile.go`) marks the orphaned job `interrupted` instead of leaving it stuck `running`, and the `.dokvol-bak` path — if one was recorded — stays discoverable through the recovery API (`system/trash.go`, `GET/POST/DELETE /api/volumes/trash*`) for manual restore or purge.

### Frontend structure (`interface/`)

- SvelteKit 5 (runes), Tailwind v4, shadcn-svelte-derived components in `src/lib/components/ui/`.
- `src/lib/api.ts` — single fetch wrapper (`fetchJson`) for all backend calls; on a 401 it transparently attempts a refresh-token exchange via `/api/auth/refresh` and retries once before forcing logout/redirect to `/login`. Backend errors are surfaced as `ApiError` carrying the same `error_code` from `system.APIError`.
- `src/lib/stores/auth.svelte.ts` — auth state (access/refresh token, current user) persisted to `localStorage`; derived stores `isLoggedIn`, `isAdmin`, `passwordChangeRequired` drive route guards and the admin-only UI.
- `src/lib/i18n/` — simple translation dictionaries (`en.ts`/`fr.ts`); the codebase mixes English and French comments/strings, so match the existing convention locally rather than translating wholesale.
- Routes under `src/routes/` mirror backend resources 1:1 (`volumes`, `drives`, `applications/[name]`, `backup/{targets,schedules,jobs,restore}`, `stats/*`, `history`, `preferences`, `login`/`register`/`change-password`).

## Auth notes

- First boot: if no admin user exists in the DB, `bootstrapAdmin` in `cmd/dokvol/server.go` requires `ADMIN_USERNAME`/`ADMIN_PASSWORD` env vars and fatally exits if unset — there is no auto-generated fallback.
- JWT secret/expiry are configured via `JWT_SECRET`, `JWT_ACCESS_EXPIRY`, `JWT_REFRESH_EXPIRY` (see `api/.env.example`); a hardcoded dev default is used if `JWT_SECRET` is unset, so always set it explicitly outside local dev.
