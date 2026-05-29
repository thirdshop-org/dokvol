#!/bin/sh
set -e

echo "=== DokVol entrypoint ==="

# 1. rsync doit être installé (installé dans l'image, check de sécurité)
if ! command -v rsync >/dev/null 2>&1; then
    echo "FATAL: rsync is not installed"
    exit 1
fi

# 2. Le socket Docker doit être accessible
if [ ! -S /var/run/docker.sock ]; then
    echo "FATAL: Docker socket not found at /var/run/docker.sock"
    echo "  Mount it with: -v /var/run/docker.sock:/var/run/docker.sock"
    exit 1
fi

# 3. Vérifier que l'API Docker répond
if ! docker info >/dev/null 2>&1; then
    echo "FATAL: Docker daemon not reachable via socket"
    exit 1
fi

echo "=== All checks passed, starting DokVol ==="

exec "$@"
