#!/bin/sh
set -e

# Configuration
DOKVOL_IMAGE="${DOKVOL_IMAGE:-ghcr.io/thirdshop-org/dokvol}"
VERSION="${DOKVOL_VERSION:-latest}"
PORT="${DOKVOL_PORT:-8080}"

# ── Pre-flight checks ──────────────────────────────────────────────

# Vérification root
if [ "$(id -u)" != "0" ]; then
    echo "❌ Erreur : Ce script doit être exécuté en tant que root (utilisez sudo)."
    exit 1
fi

# Vérification Linux
if [ "$(uname)" = "Darwin" ]; then
    echo "❌ Erreur : DokVol ne supporte que Linux pour la gestion directe des volumes."
    exit 1
fi

# Vérification Docker
if ! command -v docker >/dev/null; then
    echo "🐳 Docker n'est pas installé. Installation en cours..."
    curl -fsSL https://get.docker.com | sh
fi

# ── Pull image ─────────────────────────────────────────────────────

echo "      _       _                 _ "
echo "   __| | ___  | | ____   ___   | |"
echo "  / _\` |/ _ \| |/ /\ \ / / _ \| |"
echo " | (_| | (_) ||   <  \ V / (_) | |"
echo "  \__,_|\___/ |_|\_\  \_/ \___/|_|"
echo ""
echo "🚀 Installation de DokVol ${VERSION}..."

echo "📥 Pulling ${DOKVOL_IMAGE}:${VERSION} ..."
docker pull "${DOKVOL_IMAGE}:${VERSION}"

# ── Nettoyage ancien container ─────────────────────────────────────

docker stop dokvol 2>/dev/null || true
docker rm dokvol 2>/dev/null || true

# ── Run (Exécution en tant que Root) ───────────────────────────────

echo "⚡ Démarrage du conteneur..."

# Note: --privileged et --user root garantissent l'accès total au FS pour rsync
docker run -d \
    --name dokvol \
    --restart unless-stopped \
    --privileged \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v /var/lib/docker/volumes:/var/lib/docker/volumes:rslave \
    -v /mnt:/mnt:rslave \
    -v /etc/dokvol:/etc/dokvol \
    -p ${PORT}:8080 \
    "${DOKVOL_IMAGE}:${VERSION}"

echo ""
echo "✅ DokVol ${VERSION} est installé et s'exécute sur le port ${PORT}"
echo "👉 Accédez à l'interface : http://$(curl -s ifconfig.me):${PORT}"
