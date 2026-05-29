# DokVol — Containerisation du backend : analyse et demande d'avis

## Contexte

DokVol est un outil Go qui migre les volumes Docker d'un disque à un autre. Architecture :

- **Backend** : Go + Gin + SQLite, tourne sur le host à côté du daemon Docker
- **Frontend** : Svelte, interface web de gestion
- **Fonctionnement** : stop container → rsync les données → verify checksum → symlink l'ancien chemin vers le nouveau → restart container

Le backend tourne actuellement **nativement sur le host** (root). Objectif : le **Dockeriser** pour bénéficier de l'isolation, de la reproductibilité, et du cycle de vie container.

## Problèmes à résoudre

### 1. Dépendance rsync

Actuellement `exec.Command("rsync", "-av", "--delete", src, dst)` — aucun check si rsync est installé. Si absent, le job échoue silencieusement.

**Solution proposée** : Installer rsync dans le Dockerfile (`apk add rsync`), version contrôlée par l'image. Check de présence dans l'entrypoint.

### 2. Détection des disques hôtes

`drives.go` utilise `gopsutil/disk.Partitions(false)` qui lit `/proc/partitions` et `/sys/block`. Dans un container standard, ces filesystems sont isolés → les disques hôtes sont invisibles.

**Options envisagées** :

| Option | Description | Avantages | Inconvénients |
|--------|-------------|-----------|---------------|
| **A — Config statique** | Fichier JSON listant les mountpoints disponibles | Simple, fiable, pas de dépendance au host FS | Perd la détection automatique |
| **B — Host proc/sys bind** | `-v /proc:/host/proc:ro` + adapter le code | Garde la détection auto | Fragile, dépend du mount namespace |
| **C — API Docker** | Inspecter les containers pour déduire les mountpoints | Zéro bind FS host | Ne détecte que les disques déjà utilisés |

### 3. Vérification d'ownership `dokvol` user

`system.go` check que `.dokvol/metadata.json` appartient à l'utilisateur `dokvol`. Dans un container, cet utilisateur n'existe pas.

**Solution** : Skip le check si `os.Geteuid() == 0`, ou créer l'utilisateur dans l'entrypoint.

### 4. Permissions pour les opérations filesystem

Le container doit pouvoir :
- Lire/écrire/supprimer dans `/var/lib/docker/volumes/<name>/_data`
- Créer des symlinks dans `/var/lib/docker/volumes/`
- Lire/écrire sur les disques de destination

**Solution** : `docker run --privileged` (le plus simple pour un outil système).

### 5. Accès au daemon Docker

Nécessaire pour stop/start/inspect les containers.

**Solution** : Bind-mount `/var/run/docker.sock`.

## Architecture cible

```
Dockerfile:
  FROM golang:1.23-alpine AS builder
  ... build ...

  FROM alpine:3.19
  RUN apk add --no-cache rsync ca-certificates tzdata
  COPY --from=builder /build/dokvol /usr/local/bin/
  COPY entrypoint.sh /
  ENTRYPOINT ["/entrypoint.sh"]
  CMD ["dokvol"]

entrypoint.sh (s'exécute dans le container au démarrage, avant le binaire Go) :
  - Vérifier que rsync est installé (command -v rsync)
  - Vérifier que le socket Docker est accessible ([ -S /var/run/docker.sock ])
  - Créer l'utilisateur dokvol si nécessaire
  - Vérifier que les mountpoints configurés existent ([ -d /mnt/sda ])
  - exec su-exec dokvol /usr/local/bin/dokvol

docker run :
  --privileged
  -v /var/run/docker.sock:/var/run/docker.sock
  -v /var/lib/docker/volumes:/var/lib/docker/volumes:rslave
  -v /mnt:/mnt:rslave
  (et/ou config statique des drives)
```

## Questions pour la deuxième voie

1. **Détection des drives** : Option A (config statique), B (host proc), ou C (API Docker) — laquelle recommanderais-tu et pourquoi ?

2. **Privileged vs capabilities fines** : `--privileged` est la solution de facilité. Serait-il possible et raisonnable de le remplacer par une combinaison de `--cap-add DAC_OVERRIDE --cap-add DAC_READ_SEARCH --cap-add SYS_ADMIN` ?

3. **Stockage des drives** : Si on choisit la config statique, quel format recommanderais-tu ? Un fichier JSON monté en volume, des variables d'environnement, ou une entrée en base SQLite ?

4. **Sécurité** : Un container privilégié avec le socket Docker et `/var/lib/docker/volumes/` — quels sont les risques et comment les mitiger ?

5. **Entrypoint design** : Faut-il que le binaire Go droppe ses privilèges après l'init (su-exec) ou qu'il tourne en root ? Le code Go a-t-il besoin de `os.Geteuid() == 0` à certains endroits ?
