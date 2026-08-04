package system

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dokvol/api/internal/db"
)

func TestListTrash_OnlyReturnsEntriesWithBackupPath(t *testing.T) {
	queries, _ := initStatsDB(t)
	ctx := context.Background()

	job, err := queries.CreateMigrationJob(ctx, db.CreateMigrationJobParams{ID: "job-1", AppName: "plex", Status: string(JobRunning)})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	withBackup, err := queries.CreateVolumeProgress(ctx, db.CreateVolumeProgressParams{
		JobID: job.ID, VolumeName: "plex-config", SourcePath: "/src/config", DestPath: "/dst/config", DestDrive: "/dst", Step: StepRelinking,
	})
	if err != nil {
		t.Fatalf("create volume progress: %v", err)
	}
	if err := queries.UpdateVolumeProgressBackupPath(ctx, db.UpdateVolumeProgressBackupPathParams{
		BackupPath: sql.NullString{String: "/src/config.dokvol-bak", Valid: true}, ID: withBackup.ID,
	}); err != nil {
		t.Fatalf("set backup path: %v", err)
	}

	if _, err := queries.CreateVolumeProgress(ctx, db.CreateVolumeProgressParams{
		JobID: job.ID, VolumeName: "plex-media", SourcePath: "/src/media", DestPath: "/dst/media", DestDrive: "/dst", Step: StepCompleted,
	}); err != nil {
		t.Fatalf("create volume progress: %v", err)
	}

	entries, err := ListTrash(queries)
	if err != nil {
		t.Fatalf("ListTrash: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 trash entry, got %d", len(entries))
	}
	if entries[0].VolumeName != "plex-config" || entries[0].AppName != "plex" || entries[0].BackupPath != "/src/config.dokvol-bak" {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
}

func TestRestoreTrashEntry_RejectsMissingBackup(t *testing.T) {
	queries, _ := initStatsDB(t)
	ctx := context.Background()

	job, err := queries.CreateMigrationJob(ctx, db.CreateMigrationJobParams{ID: "job-1", AppName: "plex", Status: string(JobRunning)})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	row, err := queries.CreateVolumeProgress(ctx, db.CreateVolumeProgressParams{
		JobID: job.ID, VolumeName: "plex-config", SourcePath: "/src/config", DestPath: "/dst/config", DestDrive: "/dst", Step: StepSyncing,
	})
	if err != nil {
		t.Fatalf("create volume progress: %v", err)
	}

	if err := RestoreTrashEntry(queries, row.ID); err == nil {
		t.Fatal("expected an error restoring a volume with no backup path")
	}
}

func TestRestoreTrashEntry_RefusesWhenSourceIsNotASymlink(t *testing.T) {
	lockDir = t.TempDir()
	queries, _ := initStatsDB(t)
	ctx := context.Background()

	dir := t.TempDir()
	source := filepath.Join(dir, "source") // a real directory, not a symlink
	if err := os.Mkdir(source, 0755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	backup := filepath.Join(dir, "source.dokvol-bak")
	if err := os.Mkdir(backup, 0755); err != nil {
		t.Fatalf("mkdir backup: %v", err)
	}

	job, err := queries.CreateMigrationJob(ctx, db.CreateMigrationJobParams{ID: "job-1", AppName: "plex", Status: string(JobRunning)})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	row, err := queries.CreateVolumeProgress(ctx, db.CreateVolumeProgressParams{
		JobID: job.ID, VolumeName: "plex-config", SourcePath: source, DestPath: "/dst/config", DestDrive: "/dst", Step: StepInterrupted,
	})
	if err != nil {
		t.Fatalf("create volume progress: %v", err)
	}
	if err := queries.UpdateVolumeProgressBackupPath(ctx, db.UpdateVolumeProgressBackupPathParams{
		BackupPath: sql.NullString{String: backup, Valid: true}, ID: row.ID,
	}); err != nil {
		t.Fatalf("set backup path: %v", err)
	}

	err = RestoreTrashEntry(queries, row.ID)
	if err == nil {
		t.Fatal("expected an error restoring when source is a real directory, not a symlink")
	}
	if !strings.Contains(err.Error(), "not a symlink") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPurgeTrashEntry_RemovesBackupAndClearsPath(t *testing.T) {
	lockDir = t.TempDir()
	queries, _ := initStatsDB(t)
	ctx := context.Background()

	dir := t.TempDir()
	backup := filepath.Join(dir, "source.dokvol-bak")
	if err := os.MkdirAll(backup, 0755); err != nil {
		t.Fatalf("mkdir backup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backup, "data.txt"), []byte("old data"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	job, err := queries.CreateMigrationJob(ctx, db.CreateMigrationJobParams{ID: "job-1", AppName: "plex", Status: string(JobRunning)})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	row, err := queries.CreateVolumeProgress(ctx, db.CreateVolumeProgressParams{
		JobID: job.ID, VolumeName: "plex-config", SourcePath: filepath.Join(dir, "source"), DestPath: "/dst/config", DestDrive: "/dst", Step: StepCompleted,
	})
	if err != nil {
		t.Fatalf("create volume progress: %v", err)
	}
	if err := queries.UpdateVolumeProgressBackupPath(ctx, db.UpdateVolumeProgressBackupPathParams{
		BackupPath: sql.NullString{String: backup, Valid: true}, ID: row.ID,
	}); err != nil {
		t.Fatalf("set backup path: %v", err)
	}

	if err := PurgeTrashEntry(queries, row.ID); err != nil {
		t.Fatalf("PurgeTrashEntry: %v", err)
	}

	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatalf("expected backup dir to be removed, stat err = %v", err)
	}

	got, err := queries.GetVolumeProgress(ctx, row.ID)
	if err != nil {
		t.Fatalf("get volume progress: %v", err)
	}
	if got.BackupPath.Valid {
		t.Fatalf("expected backup_path to be cleared, got %q", got.BackupPath.String)
	}
}
