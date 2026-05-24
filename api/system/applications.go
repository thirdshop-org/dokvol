package system

import (
	"fmt"
	"sync"
)

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
	mx            sync.Mutex
}

type ApplicationVolumeOptions struct {
	VolumeDetail     VolumeDetail
	DestinationDrive DriveInfo // Overright the DefaultDestinationDrive with custom values
}

type MoveStorageOptions struct {
	DefaultDestinationDrive *DriveInfo
	ApplicationVolumes      *[]ApplicationVolumeOptions
	Application             Application
}

// Move actual docker storage volume to the
func (s *System) MoveApplicationStorage(moveStorageOptions MoveStorageOptions) error {

	drives := moveStorageOptions.Application.GroupVolumesBySystemDrives()

	if len(drives) == 0 {
		return fmt.Errorf("No storage detected for this application")
	}

	if moveStorageOptions.DefaultDestinationDrive == nil && moveStorageOptions.ApplicationVolumes == nil {
		return fmt.Errorf("No configuration provided")
	}

	var volumes *[]ApplicationVolumeOptions
	var app *Application

	// If there is a default destination drive and no applications volumes config
	// Move all volumes to the destination
	if moveStorageOptions.DefaultDestinationDrive != nil {

		if moveStorageOptions.ApplicationVolumes == nil { // Ok move all volumes to the destination

			for _, application := range s.Applications {
				if application.Name == moveStorageOptions.Application.Name {

					app = &application

					var tempVolumes []ApplicationVolumeOptions

					for _, vol := range app.DockerVolumes {

						tempVolumes = append(tempVolumes, ApplicationVolumeOptions{
							VolumeDetail:     vol,
							DestinationDrive: *moveStorageOptions.DefaultDestinationDrive,
						})

					}

					volumes = &tempVolumes

					break
				}
			}

		} else {
			return fmt.Errorf("If a default destination drive is set, you can not define specific volumes destination")
		}

	} else { // DefaultDestinationDrive is not set

		if len(*moveStorageOptions.ApplicationVolumes) == len(moveStorageOptions.Application.DockerVolumes) && len(*moveStorageOptions.ApplicationVolumes) > 0 && len(moveStorageOptions.Application.DockerVolumes) > 0 {

			volumes = moveStorageOptions.ApplicationVolumes
			app = &moveStorageOptions.Application

		} else {
			return fmt.Errorf("You must provide the same number of volumes as the docker application volumes")
		}

	}

	// Check app and volumes
	if app == nil {
		return fmt.Errorf("Application doesnt exist")
	}

	if volumes == nil {
		return fmt.Errorf("Volumes doesnt exist")
	}

	appExist := false
	for _, application := range s.Applications {
		if application.Name == app.Name {
			appExist = true
		}
	}

	if !appExist {
		return fmt.Errorf("Application doesnt exist in the system")
	}

	// Check volumes
	for _, originalAppVolume := range app.DockerVolumes {

		exist := false

		for _, providedVolume := range *volumes {
			if providedVolume.VolumeDetail == originalAppVolume {
				exist = true
				break
			}
		}

		if !exist {
			return fmt.Errorf("Some volumes are missing, you must provide every volumes of the application")
		}

	}

	for _, providedVolume := range *volumes {

		exist := false
		for _, drive := range s.Drives {
			if providedVolume.DestinationDrive == drive {
				exist = true
				break
			}
		}

		if !exist {
			return fmt.Errorf("Some destinationDrive doesnt exist")
		}

	}

	return nil

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
