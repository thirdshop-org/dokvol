package system

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"dokvol/api/internal/db"
)

// terminalSteps are volume progress steps that represent a real, known
// outcome. Anything else left behind by a job whose process died is not a
// known outcome — it just means dokvol restarted before finding out.
var terminalSteps = map[string]bool{
	StepCompleted:   true,
	StepFailed:      true,
	StepInterrupted: true,
}

// ReconcileMigrationJobs runs once at startup, before anything else touches
// the database. A migration_job left with status 'running' can only mean
// the process died mid-migration (crash, OOM, forced restart, `docker rm`) —
// nothing else leaves that status behind, since runJob always moves it to
// completed/failed on any exit path. There is no way to know, from the
// database alone, whether rsync/relink/container-restart completed partway
// through, so this does not attempt to resume or guess an outcome. It marks
// the job and its not-yet-terminal volumes 'interrupted' and, when a
// backup_path was recorded, surfaces it in the error message so an operator
// knows exactly where to look before trusting either copy of the data.
func ReconcileMigrationJobs(queries *db.Queries) error {
	ctx := context.Background()

	jobs, err := queries.ListRunningMigrationJobs(ctx)
	if err != nil {
		return fmt.Errorf("list running migration jobs: %w", err)
	}

	for _, job := range jobs {
		rows, err := queries.ListVolumeProgressByJob(ctx, job.ID)
		if err != nil {
			log.Printf("reconcile: list progress for job %s: %s", job.ID, err)
			continue
		}

		for _, row := range rows {
			if terminalSteps[row.Step] {
				continue
			}

			hint := fmt.Sprintf("dokvol restarted mid-migration at step '%s'", row.Step)
			if row.BackupPath.Valid && row.BackupPath.String != "" {
				hint += fmt.Sprintf("; pre-migration data may still be at '%s' — verify before deleting anything", row.BackupPath.String)
			}

			if err := queries.MarkVolumeProgressInterrupted(ctx, db.MarkVolumeProgressInterruptedParams{
				ErrorMessage: sql.NullString{String: hint, Valid: true},
				ID:           row.ID,
			}); err != nil {
				log.Printf("reconcile: mark volume '%s' (job %s) interrupted: %s", row.VolumeName, job.ID, err)
				continue
			}
			log.Printf("reconcile: job %s volume '%s': %s", job.ID, row.VolumeName, hint)
		}

		if err := queries.UpdateMigrationJobStatus(ctx, db.UpdateMigrationJobStatusParams{
			Status: string(JobInterrupted),
			ID:     job.ID,
		}); err != nil {
			log.Printf("reconcile: mark job %s interrupted: %s", job.ID, err)
		}
	}

	if len(jobs) > 0 {
		log.Printf("reconcile: marked %d orphaned migration job(s) as interrupted — see /api/volumes/migrate for details", len(jobs))
	}

	return nil
}
