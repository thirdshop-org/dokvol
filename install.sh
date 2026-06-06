#!/bin/sh
set -e

ACTION="${1:-install}"

# Configuration
DOKVOL_IMAGE="${DOKVOL_IMAGE:-ghcr.io/thirdshop-org/dokvol}"
VERSION="${DOKVOL_VERSION:-latest}"
PORT="${DOKVOL_PORT:-8080}"

echo "      _       _                 _ "
echo "   __| | ___  | | ____   ___   | |"
echo "  / _\` |/ _ \| |/ /\ \ / / _ \| |"
echo " | (_| | (_) ||   <  \ V / (_) | |"
echo "  \__,_|\___/ |_|\_\  \_/ \___/|_|"
echo ""

case "$ACTION" in
  install)
    echo "🚀 Installation de DokVol ${VERSION}..."

    if [ "$(id -u)" != "0" ]; then
        echo "❌ Erreur : Ce script doit être exécuté en tant que root (utilisez sudo)."
        exit 1
    fi
    if [ "$(uname)" = "Darwin" ]; then
        echo "❌ Erreur : DokVol ne supporte que Linux pour la gestion directe des volumes."
        exit 1
    fi
    if ! command -v docker >/dev/null; then
        echo "🐳 Docker n'est pas installé. Installation en cours..."
        curl -fsSL https://get.docker.com | sh
    fi

    if docker inspect dokvol >/dev/null 2>&1; then
        echo "❌ Le conteneur 'dokvol' existe déjà."
        echo "   Pour mettre à jour : sudo sh ./install.sh update"
        exit 1
    fi

    echo "📥 Pulling ${DOKVOL_IMAGE}:${VERSION} ..."
    docker pull "${DOKVOL_IMAGE}:${VERSION}"

    echo "⚡ Démarrage du conteneur..."
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
    echo "   Image : ${DOKVOL_IMAGE}:${VERSION}"
    echo "👉 Accédez à l'interface : http://$(curl -s ifconfig.me):${PORT}"
    ;;

  update|upgrade)
    echo "🚀 Mise à jour de DokVol vers ${VERSION}..."

    docker rm -f dokvol 2>/dev/null || true

    echo "📥 Pulling ${DOKVOL_IMAGE}:${VERSION} ..."
    docker pull "${DOKVOL_IMAGE}:${VERSION}"

    echo "⚡ Démarrage du nouveau conteneur..."
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
    echo "✅ DokVol mis à jour vers ${VERSION}"
    echo "👉 Accédez à l'interface : http://$(curl -s ifconfig.me):${PORT}"
    ;;

  *)
    echo "Usage: $0 [install|update]"
    echo ""
    echo "  install    Installer DokVol (défaut)"
    echo "  update     Mettre à jour DokVol vers la dernière version"
    exit 1
    ;;
esac
