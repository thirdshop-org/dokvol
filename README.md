# DokVol

> **Docker Volume Manager** — Migrate, monitor, and manage Docker volumes across drives.

DokVol is a lightweight appliance that scans Docker volumes, tracks disk usage, and lets you migrate data between drives — without touching container configurations.

## How it works

1. **Scan** — Lists all Docker volumes and bind mounts, maps them to physical drives
2. **Monitor** — Tracks disk usage per volume over time (stats collector)
3. **Migrate** — Moves volume data between drives via rsync with automatic rollback

## Quick install

```sh
curl -sSL https://ghcr.io/thirdshop-org/dokvol/install.sh | sh
```

Or with a specific version:

```sh
DOKVOL_VERSION=1.2.3 curl -sSL https://ghcr.io/thirdshop-org/dokvol/install.sh | sh
```

The script will:
- Detect your OS (Linux only)
- Install Docker if missing
- Pull the DokVol image
- Start the container on port 8080

### Manual run

```sh
docker run -d \
  --name dokvol \
  --restart unless-stopped \
  --privileged \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /var/lib/docker/volumes:/var/lib/docker/volumes:rslave \
  -v /mnt:/mnt:rslave \
  -v /etc/dokvol:/etc/dokvol \
  -p 8080:8080 \
  ghcr.io/thirdshop-org/dokvol:latest
```

## API

All endpoints are under `/api`.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | System health |
| GET | `/api/drives` | List available drives |
| POST | `/api/drives/init` | Initialize a drive for DokVol |
| GET | `/api/drives/health` | Check drive health |
| GET | `/api/volumes` | List Docker volumes |
| POST | `/api/volumes/migrate` | Start a volume migration |
| GET | `/api/volumes/migrate` | List active migrations |
| GET | `/api/volumes/migrate/:id` | Get migration status |
| GET | `/api/applications` | List applications with volumes |
| GET | `/api/preferences` | Get user preferences |
| PUT | `/api/preferences` | Update user preferences |
| GET | `/api/stats/volumes` | Volume usage history |
| GET | `/api/stats/drives` | Drive usage history |
| GET | `/api/stats/applications` | Application volume history |

## Development

### Prerequisites

- Go 1.26+
- Bun (for frontend)
- Docker

### Build locally

```sh
# Build the Docker image (frontend + backend in one image)
docker build -t dokvol:dev .

# Run
docker run -d \
  --name dokvol \
  --privileged \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /var/lib/docker/volumes:/var/lib/docker/volumes:rslave \
  -v /mnt:/mnt:rslave \
  -v /etc/dokvol:/etc/dokvol \
  -p 8080:8080 \
  dokvol:dev
```

### Frontend only (dev mode)

```sh
cd interface
bun install
bun run dev  # starts on :5173, proxies /api to localhost:8080
```

### Backend only (dev mode)

```sh
cd api
go run ./cmd/server/
```

## Architecture

DokVol ships as a **single Docker image** containing:

- **Backend** — Go/Gin API server (port 8080)
- **Frontend** — SvelteKit SPA, built to static files, served by Gin

Both are bundled together for zero-config deployment.

```
┌──────────────────────────────────────┐
│  dokvol:latest                       │
│  ┌────────────┐  ┌────────────────┐  │
│  │ Go/Gin API │  │ SvelteKit SPA  │  │
│  │ :8080/api  │  │ :8080 /*       │  │
│  └────────────┘  └────────────────┘  │
│       │                               │
│       ▼                               │
│  ┌──────────┐                         │
│  │ SQLite DB│ (migrations, stats)     │
│  └──────────┘                         │
└──────────────────────────────────────┘
```

## Versioning

Tags follow [SemVer](https://semver.org/).

| Tag | Description |
|-----|-------------|
| `latest` | Latest stable release |
| `1.2.3` | Exact version |
| `1.2` | Latest patch of minor |
| `sha-xxxxx` | Per-commit build |

Push a tag to trigger CI:

```sh
git tag v1.2.3
git push --tags
```

GitHub Actions builds and pushes `:latest`, `:1.2.3`, `:1.2`, and `:1` to `ghcr.io/thirdshop-org/dokvol`.
