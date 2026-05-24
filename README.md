Excellent choix. **DokVol** est court, mémorisable, et s'inscrit parfaitement dans l'écosystème "Dok-quelque-chose". 

Voici ta feuille de route (Roadmap) pour transformer cette idée en un projet de niveau Senior.

---

# 🧊 Roadmap : DokVol
> **Le gestionnaire de stockage intelligent pour Docker.**

## 1. La Vision du Projet
**Problème :** Dans un serveur Docker (type VPS avec Dokploy), le stockage est rigide. On définit un chemin (`/var/lib/docker/...`), et si le disque est plein ou si on veut déplacer les données sur un autre volume (Block Storage, HDD externe), c'est une manipulation manuelle risquée (arrêt, déplacement, changement de config, redémarrage).
**Solution :** DokVol devient une couche d'abstraction. Il scanne tes volumes, surveille leur taille, et permet de les "shifter" d'un disque à l'autre en un clic sans casser tes fichiers de configuration.

---

## 2. Architecture Technique
*   **Langage :** Go (pour la performance, le binaire unique et l'accès système).
*   **Interaction Docker :** Docker SDK for Go (pas de commandes shell `docker ps`).
*   **Moteur de transfert :** `rsync` (appelé par Go) pour la fiabilité et la reprise sur erreur.
*   **Interface :** 
    *   Phase 1 : CLI (Ligne de commande).
    *   Phase 2 : Dashboard Web (SvelteKit ou React + Tailwind) pour coller au style Dokploy.

---

## 3. Jalons de Développement (Feuille de Route)

### Phase 1 : L'Explorateur (MVP - Minimum Viable Product)
*Objectif : Voir ce qu'il se passe sur le serveur.*
*   **Scanner Docker :** Utiliser le SDK pour lister tous les conteneurs et identifier leurs points de montage (`Mounts` et `Volumes`).
*   **Mapping Physique :** Lier chaque volume Docker au chemin réel sur le disque (`/var/lib/docker/volumes/...` ou bind mounts).
*   **Calcul de taille :** Analyser récursivement la taille de chaque dossier de volume (en Go) pour identifier les "gros consommateurs".
*   **CLI Output :** Une commande `dokvol list` qui affiche un joli tableau : Conteneur | Volume | Chemin Réel | Taille | Disque.

### Phase 2 : Le Magicien (Le "Move")
*Objectif : Déplacer les données proprement.*
*   **Abstraction par Symlink :** Implémenter la logique suivante :
    1. L'utilisateur crée un "Point DokVol" (ex: `/mnt/dokvol/db_data`).
    2. Ce point est un lien symbolique vers le stockage réel.
*   **Workflow de Migration Automatisé :**
    1. **Stop :** Arrêter le conteneur via le SDK.
    2. **Sync :** Copier les données vers la nouvelle destination (ex: de `/sda` vers `/sdb`) avec `rsync`.
    3. **Verify :** Vérifier l'intégrité (checksum).
    4. **Relink :** Mettre à jour le lien symbolique pour pointer vers le nouveau disque.
    5. **Start :** Relancer le conteneur.
*   **Rollback :** Si la copie échoue, le conteneur redémarre sur l'ancienne destination.

### Phase 3 : Le Gardien (Monitoring & Alertes)
*Objectif : Prévenir avant que ça ne plante.*
*   **Thresholds :** Définir des seuils (ex: "Alerte-moi si le volume de MariaDB dépasse 80% du disque actuel").
*   **Auto-Cleanup :** Détecter et supprimer les volumes "orphelins" (ceux qui ne sont plus attachés à aucun conteneur).
*   **Logs :** Historique de tous les déplacements de données pour l'audit.

### Phase 4 : L'Interface DokVol (Style Dokploy)
*Objectif : Rendre le projet "Senior-ready" visuellement.*
*   **Dashboard :** Visualisation graphique des disques (camemberts de remplissage).
*   **Drag & Drop :** "Glisser" un service d'un disque vers un autre pour déclencher la migration.
*   **API REST :** Permettre à Dokploy (ou d'autres outils) d'appeler DokVol via des endpoints sécurisés.

---

## 4. Les "Must-Have" (Critères de qualité Senior)

1.  **Safety First :** Ne jamais supprimer les données sources avant que la destination ne soit confirmée à 100%.
2.  **Concurrency :** Utiliser les *Goroutines* pour scanner plusieurs disques en même temps sans bloquer l'UI.
3.  **Logging Propre :** Utiliser une librairie comme `zerolog` ou `slog` pour avoir des logs structurés en JSON (facile à lire pour d'autres outils).
4.  **Tests Unitaires :** Tester la logique de calcul de taille et de gestion des symlinks (très important pour un outil système).

---

## 5. Pourquoi ce projet va booster ton CV ?

*   **Maîtrise système :** Tu prouves que tu sais gérer les systèmes de fichiers (FS), les permissions Linux et les points de montage.
*   **Écosystème Docker :** Tu montres une expertise profonde du moteur Docker, au-delà du simple `docker-compose up`.
*   **Fiabilité :** Déplacer des données de production est la tâche la plus stressante d'un DevOps. Si ton outil le fait de manière sûre, tu gagnes une crédibilité immense.

