package system

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type ApplicationVolumeOptions struct {
	VolumeDetail     VolumeDetail
	DestinationDrive DriveInfo // Overright the DefaultDestinationDrive with custom values
}

type MoveStorageOptions struct {
	DefaultDestinationDrive *DriveInfo
	ApplicationVolumes      *[]ApplicationVolumeOptions
	Application             Application
}

func (s *System) MoveApplicationStorage(opts MoveStorageOptions) error {

	// 1. Résoudre
	app, volumes, err := s.resolveVolumesAndApp(opts)
	if err != nil {
		return fmt.Errorf("resolve: %w", err)
	}

	// 2. Valider
	if err := s.validateMigration(app, volumes); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	// 3. Migrer
	for _, vol := range *volumes {
		if err := s.migrateVolume(*app, vol); err != nil {
			return fmt.Errorf("migrate volume '%s': %w", vol.VolumeDetail.Name, err)
		}
	}

	return nil
}

func (s *System) migrateVolume(app Application, vol ApplicationVolumeOptions) error {

	sourcePath := vol.VolumeDetail.Source
	destDrive := vol.DestinationDrive

	// Dossier de destination dans .dokvol du drive cible
	destPath := filepath.Join(destDrive.Mountpoint, DOKVOL_FOLDER, app.Name, vol.VolumeDetail.Name)

	// 1. STOP — Arrêter le conteneur
	if err := s.stopContainer(app.Name); err != nil {
		return fmt.Errorf("stop failed: %w", err)
	}

	// 2. SYNC — Copier avec rsync
	if err := s.rsync(sourcePath, destPath); err != nil {
		s.startContainer(app.Name) // rollback
		return fmt.Errorf("sync failed: %w", err)
	}

	// 3. VERIFY — Checksum
	if err := s.verifyChecksum(sourcePath, destPath); err != nil {
		s.startContainer(app.Name) // rollback
		return fmt.Errorf("verify failed: %w", err)
	}

	// 4. RELINK — Remplacer sourcePath par un symlink
	if err := s.relink(sourcePath, destPath); err != nil {
		s.startContainer(app.Name) // rollback
		return fmt.Errorf("relink failed: %w", err)
	}

	// 5. START — Relancer
	if err := s.startContainer(app.Name); err != nil {
		return fmt.Errorf("start failed: %w", err)
	}

	return nil
}

func (s *System) resolveVolumesAndApp(opts MoveStorageOptions) (*Application, *[]ApplicationVolumeOptions, error) {

	// Trouver l'app dans le système
	var app *Application
	for _, a := range s.Applications {
		if a.Name == opts.Application.Name {
			app = &a
			break
		}
	}

	if app == nil {
		return nil, nil, NewAPIError(
			ErrAppNotFound,
			fmt.Sprintf("application '%s' not found in system", opts.Application.Name),
			map[string]any{"application": opts.Application.Name},
		)
	}

	if len(app.DockerVolumes) == 0 {
		return nil, nil, NewAPIError(
			ErrAppNoVolumes,
			fmt.Sprintf("application '%s' has no volumes", app.Name),
			map[string]any{"application": app.Name},
		)
	}

	// Cas 1 : DefaultDestinationDrive → tous les volumes vers la même destination
	if opts.DefaultDestinationDrive != nil && opts.ApplicationVolumes == nil {

		var volumes []ApplicationVolumeOptions

		for _, vol := range app.DockerVolumes {
			volumes = append(volumes, ApplicationVolumeOptions{
				VolumeDetail:     vol,
				DestinationDrive: *opts.DefaultDestinationDrive,
			})
		}

		return app, &volumes, nil
	}

	// Cas 2 : ApplicationVolumes défini → migration par volume
	if opts.DefaultDestinationDrive == nil && opts.ApplicationVolumes != nil {

		if len(*opts.ApplicationVolumes) == 0 {
			return nil, nil, NewAPIError(
				ErrMigrationNoDest,
				"ApplicationVolumes is empty",
				nil,
			)
		}

		if len(*opts.ApplicationVolumes) != len(app.DockerVolumes) {
			return nil, nil, NewAPIError(
				ErrMigrationVolMismatch,
				fmt.Sprintf("volume count mismatch: got %d, expected %d",
					len(*opts.ApplicationVolumes),
					len(app.DockerVolumes),
				),
				map[string]any{
					"provided": len(*opts.ApplicationVolumes),
					"expected": len(app.DockerVolumes),
				},
			)
		}

		return app, opts.ApplicationVolumes, nil
	}

	// Cas 3 : Les deux définis → ambiguïté
	if opts.DefaultDestinationDrive != nil && opts.ApplicationVolumes != nil {
		return nil, nil, NewAPIError(
			ErrMigrationAmbiguous,
			"cannot set both DefaultDestinationDrive and ApplicationVolumes",
			nil,
		)
	}

	// Cas 4 : Rien défini
	return nil, nil, NewAPIError(
		ErrMigrationNoDest,
		"no destination provided: set DefaultDestinationDrive or ApplicationVolumes",
		nil,
	)
}

func (s *System) validateMigration(app *Application, volumes *[]ApplicationVolumeOptions) error {

	// 1. Vérifier que chaque volume fourni correspond à un volume de l'app
	for _, provided := range *volumes {

		found := false
		for _, appVol := range app.DockerVolumes {
			if provided.VolumeDetail == appVol {
				found = true
				break
			}
		}

		if !found {
			return NewAPIError(
				ErrMigrationVolNotFound,
				fmt.Sprintf("volume '%s' does not belong to application '%s'", provided.VolumeDetail.Name, app.Name),
				map[string]any{
					"volume":      provided.VolumeDetail.Name,
					"application": app.Name,
				},
			)
		}
	}

	// 2. Vérifier que chaque volume de l'app est couvert
	for _, appVol := range app.DockerVolumes {

		covered := false
		for _, provided := range *volumes {
			if provided.VolumeDetail == appVol {
				covered = true
				break
			}
		}

		if !covered {
			return NewAPIError(
				ErrMigrationNoDest,
				fmt.Sprintf("volume '%s' of application '%s' has no destination", appVol.Name, app.Name),
				map[string]any{
					"volume":      appVol.Name,
					"application": app.Name,
				},
			)
		}
	}

	// 3. Vérifier que les drives de destination existent dans le système
	for _, provided := range *volumes {

		driveExists := false
		for _, drive := range s.Drives {
			if provided.DestinationDrive == drive {
				driveExists = true
				break
			}
		}

		if !driveExists {
			return NewAPIError(
				ErrDriveNotFound,
				fmt.Sprintf("destination drive '%s' does not exist in system", provided.DestinationDrive.Device),
				map[string]any{
					"device": provided.DestinationDrive.Device,
				},
			)
		}
	}

	// 4. Vérifier que la destination n'est pas la même que la source
	for _, provided := range *volumes {

		sourceDrive := s.getDriveForPath(provided.VolumeDetail.Source)

		if sourceDrive != nil && *sourceDrive == provided.DestinationDrive {
			return NewAPIError(
				ErrMigrationSameDrive,
				fmt.Sprintf("volume '%s' is already on drive '%s'", provided.VolumeDetail.Name, provided.DestinationDrive.Device),
				map[string]any{
					"volume": provided.VolumeDetail.Name,
					"drive":  provided.DestinationDrive.Device,
				},
			)
		}
	}

	// 5. Vérifier l'espace disponible sur chaque drive de destination
	if err := s.checkDiskSpace(volumes); err != nil {
		return err
	}

	return nil
}

func (s *System) relink(sourcePath, destPath string) error {

	// Supprimer l'ancien dossier source
	if err := os.RemoveAll(sourcePath); err != nil {
		return NewAPIError(
			ErrMigrationRelinkFailed,
			fmt.Sprintf("failed to remove source: %s", err),
			map[string]any{"path": sourcePath},
		)
	}

	// Créer le symlink : sourcePath → destPath
	if err := os.Symlink(destPath, sourcePath); err != nil {
		return NewAPIError(
			ErrMigrationRelinkFailed,
			fmt.Sprintf("failed to create symlink: %s", err),
			map[string]any{
				"source": sourcePath,
				"target": destPath,
			},
		)
	}

	return nil
}

func (s *System) stopContainer(appName string) error {

	ctx := context.Background()

	// Timeout avant kill forcé
	timeout := 30 // secondes

	if _, err := s.docker.ContainerStop(ctx, appName, client.ContainerStopOptions{
		Timeout: &timeout,
	}); err != nil {
		return NewAPIError(
			ErrContainerStopFailed,
			fmt.Sprintf("failed to stop container '%s': %s", appName, err),
			map[string]any{"container": appName},
		)
	}

	// Attendre que le conteneur soit vraiment arrêté
	result := s.docker.ContainerWait(ctx, appName, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})

	select {
	case err := <-result.Error:
		if err != nil {
			return NewAPIError(
				ErrContainerStopFailed,
				fmt.Sprintf("error waiting for container stop: %s", err),
				map[string]any{"container": appName},
			)
		}
	case <-result.Result:
		// OK
	}

	return nil
}

func (s *System) startContainer(appName string) error {

	ctx := context.Background()

	if _, err := s.docker.ContainerStart(ctx, appName, client.ContainerStartOptions{}); err != nil {
		return NewAPIError(
			ErrContainerStartFailed,
			fmt.Sprintf("failed to start container '%s': %s", appName, err),
			map[string]any{"container": appName},
		)
	}

	// Attendre que le conteneur soit healthy ou running
	if err := s.waitForContainer(appName); err != nil {
		return fmt.Errorf("container started but not healthy: %w", err)
	}

	return nil
}

func (s *System) waitForContainer(appName string) error {

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return NewAPIError(
				ErrContainerTimeout,
				fmt.Sprintf("timeout waiting for container '%s' to be running", appName),
				map[string]any{"container": appName},
			)
		default:
			inspect, err := s.docker.ContainerInspect(context.Background(), appName, client.ContainerInspectOptions{})
			if err != nil {
				return NewAPIError(
					ErrContainerTimeout,
					fmt.Sprintf("failed to inspect container '%s': %s", appName, err),
					map[string]any{"container": appName},
				)
			}

			switch inspect.Container.State.Status {
			case "running":
				if inspect.Container.State.Health == nil {
					return nil
				}
				if inspect.Container.State.Health.Status == "healthy" {
					return nil
				}
			case "exited", "dead":
				return NewAPIError(
					ErrContainerStopFailed,
					fmt.Sprintf("container '%s' exited unexpectedly", appName),
					map[string]any{"container": appName},
				)
			}

			time.Sleep(2 * time.Second)
		}
	}
}

// Trouver le drive d'un path donné
func (s *System) getDriveForPath(path string) *DriveInfo {

	var bestMatch *DriveInfo
	bestLen := 0

	for _, drive := range s.Drives {
		if strings.HasPrefix(path, drive.Mountpoint) && len(drive.Mountpoint) > bestLen {
			driveCopy := drive
			bestMatch = &driveCopy
			bestLen = len(drive.Mountpoint)
		}
	}

	return bestMatch
}

// Vérifier l'espace disponible sur les drives de destination
func (s *System) checkDiskSpace(volumes *[]ApplicationVolumeOptions) error {

	// Calculer l'espace nécessaire par drive
	spaceNeeded := make(map[string]int64) // mountpoint → bytes

	for _, vol := range *volumes {
		size, err := dirSize(vol.VolumeDetail.Source)
		if err != nil {
			return fmt.Errorf("failed to get size of '%s': %w", vol.VolumeDetail.Source, err)
		}
		spaceNeeded[vol.DestinationDrive.Mountpoint] += size
	}

	// Vérifier pour chaque drive
	for mountpoint, needed := range spaceNeeded {
		available, err := availableDiskSpace(mountpoint)
		if err != nil {
			return fmt.Errorf("failed to get available space on '%s': %w", mountpoint, err)
		}

		if needed > available {
			return NewAPIError(
				ErrMigrationDiskSpace,
				fmt.Sprintf("not enough space on '%s'", mountpoint),
				map[string]any{
					"drive":           mountpoint,
					"needed_bytes":    needed,
					"available_bytes": available,
				},
			)
		}
	}

	return nil
}

func (s *System) rsync(sourcePath, destPath string) error {

	// Créer le dossier de destination si il n'existe pas
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// rsync -a = archive (permissions, timestamps, récursif)
	// rsync -v = verbose
	// --delete = supprimer les fichiers qui n'existent plus dans la source
	// Trailing slash sur sourcePath = copier le contenu, pas le dossier lui-même
	cmd := exec.Command("rsync", "-av", "--delete",
		sourcePath+"/",
		destPath+"/",
	)

	// Capturer stdout et stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return NewAPIError(
			ErrMigrationSyncFailed,
			fmt.Sprintf("rsync failed: %s", stderr.String()),
			map[string]any{
				"source":      sourcePath,
				"destination": destPath,
				"stderr":      stderr.String(),
			},
		)
	}

	return nil
}

func (s *System) verifyChecksum(sourcePath, destPath string) error {

	return filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {

		if err != nil {
			return err
		}

		// Ignorer les dossiers
		if info.IsDir() {
			return nil
		}

		// Construire le chemin équivalent dans la destination
		relativePath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		destFilePath := filepath.Join(destPath, relativePath)

		// Comparer les checksums
		if err := compareFileChecksum(path, destFilePath); err != nil {
			return NewAPIError(
				ErrMigrationVerifyFailed,
				fmt.Sprintf("checksum mismatch for '%s'", relativePath),
				map[string]any{
					"file":  relativePath,
					"error": err.Error(),
				},
			)
		}

		return nil
	})
}

type VerifyMode int

const (
	VerifyFast     VerifyMode = iota // taille uniquement
	VerifyChecksum                   // sha256 complet
)

func (s *System) verifyTransfer(sourcePath, destPath string, mode VerifyMode) error {
	switch mode {
	case VerifyFast:
		return quickVerify(sourcePath, destPath)
	case VerifyChecksum:
		return s.verifyChecksum(sourcePath, destPath)
	}
	return nil
}

