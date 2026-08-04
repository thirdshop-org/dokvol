package system

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"dokvol/api/internal/db"
)

// TrashEntry describes a volume whose pre-migration data has not yet been
// reclaimed: either the migration is still mid-flight, or it was
// interrupted before ever getting to the cleanup step. It's "trash" in the
// sense of a recycle bin, not garbage — the data is intact at BackupPath
// until an operator explicitly restores or purges it.
type TrashEntry struct {
	VolumeProgressID int64
	JobID            string
	AppName          string
	VolumeName       string
	SourcePath       string
	DestPath         string
	DestDrive        string
	Step             string
	BackupPath       string
}

// ListTrash returns every volume with a recorded, not-yet-reclaimed backup
// path.
func ListTrash(queries *db.Queries) ([]TrashEntry, error) {
	rows, err := queries.ListVolumeProgressWithBackupPath(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list volume progress with backup path: %w", err)
	}

	entries := make([]TrashEntry, len(rows))
	for i, r := range rows {
		entries[i] = TrashEntry{
			VolumeProgressID: r.ID,
			JobID:            r.JobID,
			AppName:          r.AppName,
			VolumeName:       r.VolumeName,
			SourcePath:       r.SourcePath,
			DestPath:         r.DestPath,
			DestDrive:        r.DestDrive,
			Step:             r.Step,
			BackupPath:       r.BackupPath.String,
		}
	}
	return entries, nil
}

// RestoreTrashEntry undoes a migration: it stops whatever containers are
// currently mounting sourcePath, removes the symlink left by relink(),
// renames the pre-migration backup back into place, and restarts those
// containers. It refuses to run if sourcePath isn't actually a symlink
// (nothing to undo, or it was already restored), and takes the same
// exclusive volume lock a migration or backup would, so it can't race one.
func RestoreTrashEntry(queries *db.Queries, id int64) error {
	ctx := context.Background()

	row, err := queries.GetVolumeProgress(ctx, id)
	if err != nil {
		return fmt.Errorf("get volume progress %d: %w", id, err)
	}
	if !row.BackupPath.Valid || row.BackupPath.String == "" {
		return fmt.Errorf("volume progress %d has no backup to restore", id)
	}
	backupPath := row.BackupPath.String

	lock, err := LockVolume(row.SourcePath)
	if err != nil {
		return err
	}
	defer lock.Unlock()

	info, statErr := os.Lstat(row.SourcePath)
	switch {
	case statErr == nil && info.Mode()&os.ModeSymlink != 0:
		// expected case: undo the relink
	case os.IsNotExist(statErr):
		// sourcePath is simply missing; proceed to restore into it
	default:
		return fmt.Errorf("refusing to restore '%s': not a symlink (already restored, or migration never reached the relink step)", row.SourcePath)
	}

	writers, err := containersMountingSource(row.SourcePath)
	if err != nil {
		return fmt.Errorf("list containers using volume: %w", err)
	}

	s := System{docker: newDockerClient()}
	var stopped []string
	for _, name := range writers {
		if err := s.stopContainer(name); err != nil {
			s.startContainers(stopped)
			return fmt.Errorf("stop container '%s': %w", name, err)
		}
		stopped = append(stopped, name)
	}

	if statErr == nil {
		if err := os.Remove(row.SourcePath); err != nil {
			s.startContainers(stopped)
			return fmt.Errorf("remove symlink at '%s': %w", row.SourcePath, err)
		}
	}

	if err := os.Rename(backupPath, row.SourcePath); err != nil {
		s.startContainers(stopped)
		return fmt.Errorf("restore backup to '%s': %w", row.SourcePath, err)
	}

	if err := s.startContainers(stopped); err != nil {
		return fmt.Errorf("data restored to '%s' but failed to restart container(s): %w", row.SourcePath, err)
	}

	if err := queries.MarkVolumeProgressRestored(ctx, id); err != nil {
		log.Printf("trash: restored '%s' but failed to update volume progress %d: %s", row.SourcePath, id, err)
	}

	return nil
}

// PurgeTrashEntry permanently deletes the backup directory, freeing the
// disk. The migrated data at DestPath (and the symlink at SourcePath, if
// any) are untouched — this only discards the pre-migration copy, and
// should only be called once an operator has confirmed the migration is
// trustworthy.
func PurgeTrashEntry(queries *db.Queries, id int64) error {
	ctx := context.Background()

	row, err := queries.GetVolumeProgress(ctx, id)
	if err != nil {
		return fmt.Errorf("get volume progress %d: %w", id, err)
	}
	if !row.BackupPath.Valid || row.BackupPath.String == "" {
		return fmt.Errorf("volume progress %d has no backup to purge", id)
	}

	lock, err := LockVolume(row.SourcePath)
	if err != nil {
		return err
	}
	defer lock.Unlock()

	if err := os.RemoveAll(row.BackupPath.String); err != nil {
		return fmt.Errorf("remove backup '%s': %w", row.BackupPath.String, err)
	}

	return queries.UpdateVolumeProgressBackupPath(ctx, db.UpdateVolumeProgressBackupPathParams{
		BackupPath: sql.NullString{},
		ID:         id,
	})
}
