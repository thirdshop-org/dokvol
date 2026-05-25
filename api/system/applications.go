package system

// **Abstraction par Symlink :** Implémenter la logique suivante :
//     1. L'utilisateur crée un "Point DokVol" (ex: `/mnt/dokvol/db_data`).
//     2. Ce point est un lien symbolique vers le stockage réel.
// *   **Workflow de Migration Automatisé :**
//     1. **Stop :** Arrêter le conteneur via le SDK.
//     2. **Sync :** Copier les données vers la nouvelle destination (ex: de `/sda` vers `/sdb`) avec `rsync`.
//     3. **Verify :** Vérifier l'intégrité (checksum).
//     4. **Relink :** Mettre à jour le lien symbolique pour pointer vers le nouveau disque.
//     5. **Start :** Relancer le conteneur.
// *   **Rollback :** Si la copie échoue, le conteneur redémarre sur l'ancienne destination.

type Application struct {
	Name          string
	DockerVolumes []VolumeDetail
}

func (a *Application) GroupVolumesBySystemDrives() map[DriveInfo][]VolumeDetail {

	group := make(map[DriveInfo][]VolumeDetail)

	for _, volume := range a.DockerVolumes {
		if volume.SystemDrive == nil {
			continue
		}
		// fmt.Print(volume.SystemDrive.Device, " ---------- ", volume.SystemDrive.Mountpoint)
		group[*volume.SystemDrive] = append(group[*volume.SystemDrive], volume)
	}

	return group

}
