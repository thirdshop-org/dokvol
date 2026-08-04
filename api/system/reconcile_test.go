package system

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"dokvol/api/internal/db"
)

func TestReconcileMigrationJobs_MarksOrphanedRunningJobInterrupted(t *testing.T) {
	queries, _ := initStatsDB(t)
	ctx := context.Background()

	job, err := queries.CreateMigrationJob(ctx, db.CreateMigrationJobParams{
		ID:      "job-1",
		AppName: "plex",
		Status:  string(JobRunning),
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	inProgress, err := queries.CreateVolumeProgress(ctx, db.CreateVolumeProgressParams{
		JobID:      job.ID,
		VolumeName: "plex-config",
		SourcePath: "/var/lib/docker/volumes/plex-config/_data",
		DestPath:   "/mnt/disk2/.dokvol/plex/plex-config",
		DestDrive:  "/mnt/disk2",
		Step:       StepRelinking,
	})
	if err != nil {
		t.Fatalf("create volume progress: %v", err)
	}
	if err := queries.UpdateVolumeProgressBackupPath(ctx, db.UpdateVolumeProgressBackupPathParams{
		BackupPath: sql.NullString{String: "/var/lib/docker/volumes/plex-config/_data.dokvol-bak", Valid: true},
		ID:         inProgress.ID,
	}); err != nil {
		t.Fatalf("set backup path: %v", err)
	}

	done, err := queries.CreateVolumeProgress(ctx, db.CreateVolumeProgressParams{
		JobID:      job.ID,
		VolumeName: "plex-media",
		SourcePath: "/var/lib/docker/volumes/plex-media/_data",
		DestPath:   "/mnt/disk2/.dokvol/plex/plex-media",
		DestDrive:  "/mnt/disk2",
		Step:       StepCompleted,
	})
	if err != nil {
		t.Fatalf("create volume progress: %v", err)
	}

	if err := ReconcileMigrationJobs(queries); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	gotJob, err := queries.GetMigrationJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if gotJob.Status != string(JobInterrupted) {
		t.Fatalf("job status = %q, want %q", gotJob.Status, JobInterrupted)
	}

	rows, err := queries.ListVolumeProgressByJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("list progress: %v", err)
	}
	for _, r := range rows {
		switch r.ID {
		case inProgress.ID:
			if r.Step != StepInterrupted {
				t.Errorf("in-progress volume step = %q, want %q", r.Step, StepInterrupted)
			}
			if !strings.Contains(r.ErrorMessage.String, "plex-config/_data.dokvol-bak") {
				t.Errorf("error message %q does not mention the backup path", r.ErrorMessage.String)
			}
		case done.ID:
			if r.Step != StepCompleted {
				t.Errorf("already-completed volume step changed to %q", r.Step)
			}
		}
	}
}
