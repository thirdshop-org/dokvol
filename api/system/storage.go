package system

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
	OnProgress              ProgressFn
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

	// Vérifier l'espace disponible cumulé par disque de destination avant de
	// commencer quoi que ce soit : plusieurs volumes migrant vers le même
	// disque doivent être comptés ensemble, pas juste chacun contre le total
	// disponible (trois volumes de 60 Go passeraient tous individuellement
	// contre un disque de 100 Go).
	if err := s.checkDiskSpace(volumes); err != nil {
		return fmt.Errorf("disk space: %w", err)
	}

	// 3. Migrer
	for _, vol := range *volumes {
		if !vol.VolumeDetail.IsMigratable {
			continue
		}
		if err := s.migrateVolume(*app, vol, opts); err != nil {
			return fmt.Errorf("migrate volume '%s': %w", volumeSubDir(vol.VolumeDetail), err)
		}
	}

	return nil
}

func (s *System) migrateVolume(app Application, vol ApplicationVolumeOptions, opts MoveStorageOptions) error {

	volName := volumeSubDir(vol.VolumeDetail)
	sourcePath := vol.VolumeDetail.Source
	destDrive := vol.DestinationDrive

	// Dossier de destination dans .dokvol du drive cible
	destPath := filepath.Join(destDrive.Mountpoint, DOKVOL_FOLDER, app.Name, volName)

	// Vérifier l'espace disponible par volume avant de migrer
	size, err := dirSize(sourcePath)
	if err != nil {
		return fmt.Errorf("source size: %w", err)
	}
	available, err := availableDiskSpace(destDrive.Mountpoint)
	if err != nil {
		return fmt.Errorf("disk space: %w", err)
	}
	if size > available {
		return NewAPIError(
			ErrMigrationDiskSpace,
			fmt.Sprintf("not enough space on '%s'", destDrive.Mountpoint),
			map[string]any{
				"drive":           destDrive.Mountpoint,
				"needed_bytes":    size,
				"available_bytes": available,
			},
		)
	}

	// 1. STOP — Arrêter tous les conteneurs qui montent ce volume. Un même
	// volume peut être partagé par plusieurs conteneurs (ou un projet
	// compose) : n'arrêter que l'application migrée laisserait les autres
	// écrire pendant le rsync.
	reportProgress(opts, volName, StepStopping, 0, 0)
	writers, err := containersMountingSource(sourcePath)
	if err != nil {
		return fmt.Errorf("list containers using volume: %w", err)
	}
	if len(writers) == 0 {
		writers = []string{app.Name}
	}

	var stopped []string
	for _, name := range writers {
		if err := s.stopContainer(name); err != nil {
			s.startContainers(stopped) // best-effort rollback of what we already stopped
			return fmt.Errorf("stop failed for container '%s': %w", name, err)
		}
		stopped = append(stopped, name)
	}

	// 2. SYNC — Copier avec rsync
	totalBytes, _ := dirSize(sourcePath)
	reportProgress(opts, volName, StepSyncing, 0, totalBytes)

	// Goroutine de progression pendant rsync
	stopProgress := make(chan struct{})
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopProgress:
				return
			case <-ticker.C:
				transferred, err := dirSize(destPath)
				if err == nil {
					reportProgress(opts, volName, StepSyncing, transferred, totalBytes)
				}
			}
		}
	}()

	rsyncErr := s.rsync(sourcePath, destPath)
	close(stopProgress)
	<-progressDone

	if rsyncErr != nil {
		s.startContainers(stopped) // rollback
		return fmt.Errorf("sync failed: %w", rsyncErr)
	}

	totalAfterSync, _ := dirSize(destPath)
	if totalAfterSync > totalBytes {
		totalBytes = totalAfterSync
	}
	reportProgress(opts, volName, StepSyncing, totalBytes, totalBytes)

	// 3. VERIFY — Checksum
	reportProgress(opts, volName, StepVerifying, totalBytes, totalBytes)
	if err := s.verifyChecksum(sourcePath, destPath); err != nil {
		s.startContainers(stopped) // rollback
		return fmt.Errorf("verify failed: %w", err)
	}

	// 4. RELINK — Remplacer sourcePath par un symlink (l'original est déplacé
	// vers backupPath, pas supprimé — voir relink())
	reportProgress(opts, volName, StepRelinking, totalBytes, totalBytes)
	backupPath, err := s.relink(sourcePath, destPath)
	if err != nil {
		s.startContainers(stopped) // rollback: original data untouched, safe to restart on it
		return fmt.Errorf("relink failed: %w", err)
	}

	// 5. START — Relancer tous les conteneurs arrêtés à l'étape 1
	reportProgress(opts, volName, StepStarting, totalBytes, totalBytes)
	if err := s.startContainers(stopped); err != nil {
		return fmt.Errorf("start failed: %w (pre-migration data preserved at %s)", err, backupPath)
	}

	// Only now that the container is confirmed healthy on the new location is
	// it safe to reclaim the pre-migration copy.
	if err := os.RemoveAll(backupPath); err != nil {
		log.Printf("migration: failed to remove backup '%s' after successful migration: %s", backupPath, err)
	}

	reportProgress(opts, volName, StepCompleted, totalBytes, totalBytes)
	return nil
}

func reportProgress(opts MoveStorageOptions, volumeName, step string, transferred, total int64) {
	if opts.OnProgress != nil {
		opts.OnProgress(volumeName, step, transferred, total)
	}
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
			if !vol.IsMigratable {
				continue
			}
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

	// Construire la liste des volumes réellement migrables de l'app
	migratableAppVols := make([]VolumeDetail, 0, len(app.DockerVolumes))
	for _, v := range app.DockerVolumes {
		if v.IsMigratable {
			migratableAppVols = append(migratableAppVols, v)
		}
	}

	if len(*volumes) < len(migratableAppVols) {
		return NewAPIError(
			ErrMigrationVolMismatch,
			fmt.Sprintf("volume count mismatch: got %d, expected at least %d",
				len(*volumes),
				len(migratableAppVols),
			),
			map[string]any{
				"provided": len(*volumes),
				"expected": len(migratableAppVols),
			},
		)
	}

	// 1. Vérifier que chaque volume fourni correspond à un volume de l'app
	for _, provided := range *volumes {
		if !provided.VolumeDetail.IsMigratable {
			continue
		}

		found := false
		for _, appVol := range migratableAppVols {
			if provided.VolumeDetail.Source == appVol.Source {
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

	// 2. Vérifier que chaque volume migrable de l'app est couvert
	for _, appVol := range migratableAppVols {

		covered := false
		for _, provided := range *volumes {
			if !provided.VolumeDetail.IsMigratable {
				continue
			}
			if appVol.Source == provided.VolumeDetail.Source {
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
		if !provided.VolumeDetail.IsMigratable {
			continue
		}

		driveExists := false
		for _, drive := range s.Drives {
			if provided.DestinationDrive.Device == drive.Device {
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

		if sourceDrive != nil && sourceDrive.Device == provided.DestinationDrive.Device {
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

	return nil
}

const relinkBackupSuffix = ".dokvol-bak"

// relink swaps sourcePath for a symlink to destPath. Instead of deleting the
// original directory outright, it is renamed aside (same filesystem, atomic)
// so that a crash between the rename and the symlink creation — or a failed
// container start right after — never destroys data. The caller is
// responsible for removing the returned backup path once the migrated
// container has been confirmed healthy.
func (s *System) relink(sourcePath, destPath string) (backupPath string, err error) {

	backupPath = sourcePath + relinkBackupSuffix

	// Déplacer l'ancien dossier source de côté (rename est atomique sur un même FS)
	if err := os.Rename(sourcePath, backupPath); err != nil {
		return "", NewAPIError(
			ErrMigrationRelinkFailed,
			fmt.Sprintf("failed to move source aside: %s", err),
			map[string]any{"path": sourcePath},
		)
	}

	// Créer le symlink : sourcePath → destPath
	if err := os.Symlink(destPath, sourcePath); err != nil {
		// Revert: restore the original directory so the volume isn't left empty
		if revertErr := os.Rename(backupPath, sourcePath); revertErr != nil {
			return "", NewAPIError(
				ErrMigrationRelinkFailed,
				fmt.Sprintf("failed to create symlink (%s) and failed to restore original data: %s (original data left at %s)", err, revertErr, backupPath),
				map[string]any{
					"source": sourcePath,
					"target": destPath,
					"backup": backupPath,
				},
			)
		}
		return "", NewAPIError(
			ErrMigrationRelinkFailed,
			fmt.Sprintf("failed to create symlink: %s", err),
			map[string]any{
				"source": sourcePath,
				"target": destPath,
			},
		)
	}

	return backupPath, nil
}

// containerStopGracePeriod is how long `docker stop` waits after SIGTERM
// before falling back to SIGKILL. A bare SIGKILL (the previous behavior via
// `docker kill`) crashes databases and anything else that needs to flush and
// close on shutdown.
const containerStopGracePeriod = 30 * time.Second

func (s *System) stopContainer(appName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), containerStopGracePeriod+60*time.Second)
	defer cancel()

	// `docker stop` is a no-op (success) on an already-stopped container, so
	// no need to special-case that like the old kill+wait pair did.
	cmd := exec.CommandContext(ctx, "docker", "stop", "--time", fmt.Sprintf("%d", int(containerStopGracePeriod.Seconds())), appName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return NewAPIError(
			ErrContainerStopFailed,
			fmt.Sprintf("failed to stop container '%s': %s\n%s", appName, err, out),
			map[string]any{"container": appName},
		)
	}
	return nil
}

// startContainers restarts every container in names, continuing past
// individual failures so a rollback restarts as many writers as possible,
// and returns an aggregate error describing every one that failed.
func (s *System) startContainers(names []string) error {
	var errs []string
	for _, name := range names {
		if err := s.startContainer(name); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to start container(s): %s", strings.Join(errs, "; "))
	}
	return nil
}

func (s *System) startContainer(appName string) error {
	cmd := exec.Command("docker", "start", appName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return NewAPIError(
			ErrContainerStartFailed,
			fmt.Sprintf("failed to start container '%s': %s\n%s", appName, err, out),
			map[string]any{"container": appName},
		)
	}
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

	// Resolve symlinks so that migrated volumes (symlink → loop drive) are detected
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}

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

// diskSpaceMarginFactor requires destination drives to have some headroom
// beyond the raw bytes needed, to account for filesystem overhead
// (block/cluster rounding, metadata) and other things landing on the same
// drive between the check and the actual rsync.
const diskSpaceMarginFactor = 1.10

// Vérifier l'espace disponible sur les drives de destination
func (s *System) checkDiskSpace(volumes *[]ApplicationVolumeOptions) error {

	// Calculer l'espace nécessaire par drive
	spaceNeeded := make(map[string]int64) // mountpoint → bytes

	for _, vol := range *volumes {
		if !vol.VolumeDetail.IsMigratable {
			continue
		}
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

		neededWithMargin := int64(float64(needed) * diskSpaceMarginFactor)
		if neededWithMargin > available {
			return NewAPIError(
				ErrMigrationDiskSpace,
				fmt.Sprintf("not enough space on '%s'", mountpoint),
				map[string]any{
					"drive":              mountpoint,
					"needed_bytes":       needed,
					"needed_with_margin": neededWithMargin,
					"available_bytes":    available,
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

		// Ne comparer que les fichiers réguliers : les dossiers sont récursés
		// séparément, et les symlinks cassés / FIFOs / sockets unix (ex:
		// .s.PGSQL.5432, mysqld.sock) ne peuvent pas être hashés — os.Open
		// bloque indéfiniment sur une FIFO sans lecteur, ce qui figeait le job.
		if !info.Mode().IsRegular() {
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

