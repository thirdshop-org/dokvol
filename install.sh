#!/bin/sh
set -e

DOKVOL_IMAGE="${DOKVOL_IMAGE:-ghcr.io/ton-user/dokvol}"
VERSION="${DOKVOL_VERSION:-latest}"

# ── Pre-flight checks ──────────────────────────────────────────────

[ "$(id -u)" != "0" ] && { echo "Run as root"; exit 1; }
[ "$(uname)" = "Darwin" ] && { echo "Linux only"; exit 1; }
[ -f /.dockerenv ] && { echo "Run on host, not inside a container"; exit 1; }
command -v docker >/dev/null || { echo "Docker is not installed"; exit 1; }

# ── Pull image ─────────────────────────────────────────────────────

echo "Pulling ${DOKVOL_IMAGE}:${VERSION} ..."
docker pull "${DOKVOL_IMAGE}:${VERSION}"

# ── Stop & remove existing container ───────────────────────────────

docker stop dokvol 2>/dev/null || true
docker rm dokvol 2>/dev/null || true

# ── Run ────────────────────────────────────────────────────────────

docker run -d \
    --name dokvol \
    --restart unless-stopped \
    --privileged \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v /var/lib/docker/volumes:/var/lib/docker/volumes:rslave \
    -v /mnt:/mnt:rslave \
    -v /etc/dokvol:/etc/dokvol \
    -p 8080:8080 \
    "${DOKVOL_IMAGE}:${VERSION}"

echo "DokVol ${VERSION} installed and running on :8080 ✓"
