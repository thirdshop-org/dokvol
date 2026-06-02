#!/bin/sh
set -e

mkdir -p /var/lib/docker

dockerd \
    --host=unix:///var/run/docker.sock \
    --storage-driver=vfs \
    --data-root=/var/lib/docker \
    &
DOCKERD_PID=$!

for i in $(seq 1 30); do
    if docker info >/dev/null 2>&1; then
        exec "$@"
    fi
    sleep 1
done

echo "FATAL: dockerd not ready after 30s"
exit 1
