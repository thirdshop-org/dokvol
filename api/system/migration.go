package system

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"dokvol/api/internal/db"

	"github.com/google/uuid"
	"github.com/moby/moby/client"
)

type MigrationJobStatus string

const (
	JobPending     MigrationJobStatus = "pending"
	JobRunning     MigrationJobStatus = "running"
	JobCompleted   MigrationJobStatus = "completed"
	JobFailed      MigrationJobStatus = "failed"
	// JobInterrupted is set by the boot-time reconciler for any job still
	// marked "running" at startup — the process died mid-migration and no
	// goroutine survived to report a real outcome.
	JobInterrupted MigrationJobStatus = "interrupted"
)

const (
	StepPending   = "pending"
	StepStopping  = "stopping"
	StepSyncing   = "syncing"
	StepVerifying = "verifying"
	StepRelinking = "relinking"
	StepStarting  = "starting"
	StepCompleted = "completed"
	StepFailed    = "failed"
	// StepInterrupted is set by the boot-time reconciler on any volume whose
	// job died mid-migration. Unlike StepFailed, it does not mean the
	// operation was seen to fail — it means dokvol restarted before finding
	// out, and an operator should check BackupPath before assuming anything.
	StepInterrupted = "interrupted"
)

type VolumeRow struct {
	mu          sync.Mutex
	ID          int64
	VolumeName  string
	Step        string
	TotalBytes  int64
	Transferred int64
	SourcePath  string
	SourceDrive string
	DestPath    string
	DestDrive   string
	Error       string
	// BackupPath is where the pre-migration data was moved aside to by
	// relink() (see storage.go), non-empty from the moment relinking starts
	// until it's reclaimed after a confirmed-healthy restart. Surfaced here
	// so an interrupted job still points at recoverable data.
	BackupPath string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Job struct {
	ID          string
	AppName     string
	Status      MigrationJobStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt time.Time
	Volumes     []VolumeRow
}

type ProgressFn func(VolumeName, Step string, TransferredBytes, TotalBytes int64)

type MigrationManager struct {
	Queries *db.Queries
	mu      sync.RWMutex
	jobs    map[string]*Job
}

func NewMigrationManager(queries *db.Queries) *MigrationManager {
	return &MigrationManager{
		Queries: queries,
		jobs:    make(map[string]*Job),
	}
}

func (m *MigrationManager) StartJob(ctx context.Context, appName string, application Application, volumes []ApplicationVolumeOptions) (string, error) {
	jobID := uuid.New().String()

	_, err := m.Queries.CreateMigrationJob(ctx, db.CreateMigrationJobParams{
		ID:      jobID,
		AppName: appName,
		Status:  string(JobPending),
	})
	if err != nil {
		return "", fmt.Errorf("create job: %w", err)
	}

	now := time.Now()
	job := &Job{
		ID:        jobID,
		AppName:   appName,
		Status:    JobPending,
		CreatedAt: now,
		UpdatedAt: now,
		Volumes:   make([]VolumeRow, len(volumes)),
	}

	for i, vol := range volumes {
		volName := volumeSubDir(vol.VolumeDetail)
		destPath := filepath.Join(vol.DestinationDrive.Mountpoint, DOKVOL_FOLDER, appName, volName)

		p, err := m.Queries.CreateVolumeProgress(ctx, db.CreateVolumeProgressParams{
			JobID:      jobID,
			VolumeName: volName,
			SourcePath: vol.VolumeDetail.Source,
			DestPath:   destPath,
			DestDrive:  vol.DestinationDrive.Mountpoint,
			Step:       StepPending,
		})
		if err != nil {
			return "", fmt.Errorf("create progress: %w", err)
		}

		job.Volumes[i] = VolumeRow{
			ID:          p.ID,
			VolumeName:  volName,
			Step:        StepPending,
			SourcePath:  vol.VolumeDetail.Source,
			SourceDrive: getDriveForSourcePath(vol.VolumeDetail.Source, GetDrives()),
			DestPath:    destPath,
			DestDrive:   vol.DestinationDrive.Mountpoint,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		job.Volumes[i].mu = sync.Mutex{}
	}

	m.mu.Lock()
	m.jobs[jobID] = job
	m.mu.Unlock()

	go m.runJob(context.Background(), jobID, application, volumes)

	return jobID, nil
}

func (m *MigrationManager) GetJob(ctx context.Context, id string) (*Job, error) {
	m.mu.RLock()
	job, ok := m.jobs[id]
	m.mu.RUnlock()

	if ok {
		copy := &Job{
			ID:          job.ID,
			AppName:     job.AppName,
			Status:      job.Status,
			CreatedAt:   job.CreatedAt,
			UpdatedAt:   job.UpdatedAt,
			CompletedAt: job.CompletedAt,
			Volumes:     make([]VolumeRow, len(job.Volumes)),
		}
		for i := range job.Volumes {
			job.Volumes[i].mu.Lock()
			copy.Volumes[i] = VolumeRow{
				ID:          job.Volumes[i].ID,
				VolumeName:  job.Volumes[i].VolumeName,
				Step:        job.Volumes[i].Step,
				TotalBytes:  job.Volumes[i].TotalBytes,
				Transferred: job.Volumes[i].Transferred,
				SourcePath:  job.Volumes[i].SourcePath,
				SourceDrive: job.Volumes[i].SourceDrive,
				DestPath:    job.Volumes[i].DestPath,
				DestDrive:   job.Volumes[i].DestDrive,
				Error:       job.Volumes[i].Error,
				BackupPath:  job.Volumes[i].BackupPath,
				CreatedAt:   job.Volumes[i].CreatedAt,
				UpdatedAt:   job.Volumes[i].UpdatedAt,
			}
			job.Volumes[i].mu.Unlock()
		}
		return copy, nil
	}

	dbJob, err := m.Queries.GetMigrationJob(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}

	progressRows, err := m.Queries.ListVolumeProgressByJob(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list progress: %w", err)
	}

	job = &Job{
		ID:      dbJob.ID,
		AppName: dbJob.AppName,
		Status:  MigrationJobStatus(dbJob.Status),
		CreatedAt: func() time.Time {
			if dbJob.CreatedAt.Valid {
				return dbJob.CreatedAt.Time
			}
			return time.Time{}
		}(),
		UpdatedAt: func() time.Time {
			if dbJob.UpdatedAt.Valid {
				return dbJob.UpdatedAt.Time
			}
			return time.Time{}
		}(),
		Volumes: make([]VolumeRow, len(progressRows)),
	}

	for i, p := range progressRows {
		job.Volumes[i] = VolumeRow{
			ID:          p.ID,
			VolumeName:  p.VolumeName,
			Step:        p.Step,
			TotalBytes:  p.TotalBytes,
			Transferred: p.TransferredBytes,
			SourcePath:  p.SourcePath,
			DestPath:    p.DestPath,
			DestDrive:   p.DestDrive,
			Error:       p.ErrorMessage.String,
			BackupPath:  p.BackupPath.String,
			CreatedAt: func() time.Time {
				if p.CreatedAt.Valid {
					return p.CreatedAt.Time
				}
				return time.Time{}
			}(),
			UpdatedAt: func() time.Time {
				if p.UpdatedAt.Valid {
					return p.UpdatedAt.Time
				}
				return time.Time{}
			}(),
		}
	}

	return job, nil
}

func (m *MigrationManager) ListJobs(ctx context.Context) ([]*Job, error) {
	rows, err := m.Queries.ListJobsWithProgress(ctx)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	seen := make(map[string]bool, len(rows))
	var jobs []*Job

	for _, row := range rows {
		jobID := row.JobID
		if seen[jobID] {
			continue
		}
		seen[jobID] = true

		if mem, ok := m.jobs[jobID]; ok {
			copy := &Job{
				ID:          mem.ID,
				AppName:     mem.AppName,
				Status:      mem.Status,
				CreatedAt:   mem.CreatedAt,
				UpdatedAt:   mem.UpdatedAt,
				CompletedAt: mem.CompletedAt,
				Volumes:     make([]VolumeRow, len(mem.Volumes)),
			}
			for i := range mem.Volumes {
				mem.Volumes[i].mu.Lock()
				copy.Volumes[i] = VolumeRow{
					ID:          mem.Volumes[i].ID,
					VolumeName:  mem.Volumes[i].VolumeName,
					Step:        mem.Volumes[i].Step,
					TotalBytes:  mem.Volumes[i].TotalBytes,
					Transferred: mem.Volumes[i].Transferred,
					SourcePath:  mem.Volumes[i].SourcePath,
					DestPath:    mem.Volumes[i].DestPath,
					DestDrive:   mem.Volumes[i].DestDrive,
					Error:       mem.Volumes[i].Error,
					BackupPath:  mem.Volumes[i].BackupPath,
					CreatedAt:   mem.Volumes[i].CreatedAt,
					UpdatedAt:   mem.Volumes[i].UpdatedAt,
				}
				mem.Volumes[i].mu.Unlock()
			}
			jobs = append(jobs, copy)
			continue
		}

		job := &Job{
			ID:      jobID,
			AppName: row.AppName,
			Status:  MigrationJobStatus(row.Status),
			CreatedAt: func() time.Time {
				if row.JobCreatedAt.Valid {
					return row.JobCreatedAt.Time
				}
				return time.Time{}
			}(),
			UpdatedAt: func() time.Time {
				if row.JobUpdatedAt.Valid {
					return row.JobUpdatedAt.Time
				}
				return time.Time{}
			}(),
		}
		for _, r := range rows {
			if r.JobID != jobID {
				continue
			}
			job.Volumes = append(job.Volumes, VolumeRow{
				ID:          r.ProgressID,
				VolumeName:  r.VolumeName,
				Step:        r.Step,
				TotalBytes:  r.TotalBytes,
				Transferred: r.TransferredBytes,
				SourcePath:  r.SourcePath,
				DestPath:    r.DestPath,
				DestDrive:   r.DestDrive,
				Error:       r.ErrorMessage.String,
				BackupPath:  r.BackupPath.String,
				CreatedAt: func() time.Time {
					if r.ProgressCreatedAt.Valid {
						return r.ProgressCreatedAt.Time
					}
					return time.Time{}
				}(),
				UpdatedAt: func() time.Time {
					if r.ProgressUpdatedAt.Valid {
						return r.ProgressUpdatedAt.Time
					}
					return time.Time{}
				}(),
			})
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (m *MigrationManager) runJob(ctx context.Context, jobID string, app Application, volumes []ApplicationVolumeOptions) {
	if err := m.Queries.UpdateMigrationJobStatus(ctx, db.UpdateMigrationJobStatusParams{
		Status: string(JobRunning),
		ID:     jobID,
	}); err != nil {
		log.Printf("failed to set job %s to running: %s", jobID, err)
	}

	m.mu.Lock()
	if j, ok := m.jobs[jobID]; ok {
		j.Status = JobRunning
		j.UpdatedAt = time.Now()
	}
	m.mu.Unlock()

	drives := GetDrives()
	apps, appsErr := GetApplicationsDetails(drives)
	s := System{
		Drives:       drives,
		Applications: apps,
	}

	startedAt := time.Now()
	completedAt := startedAt

	opts := MoveStorageOptions{
		Application:        app,
		ApplicationVolumes: &volumes,
		OnProgress: func(volumeName, step string, transferred, total int64) {
			m.mu.RLock()
			job, ok := m.jobs[jobID]
			m.mu.RUnlock()
			if !ok {
				return
			}

			var vol *VolumeRow
			for i := range job.Volumes {
				if job.Volumes[i].VolumeName == volumeName {
					vol = &job.Volumes[i]
					break
				}
			}
			if vol == nil {
				return
			}

			vol.mu.Lock()
			vol.Step = step
			vol.TotalBytes = total
			vol.Transferred = transferred
			vol.mu.Unlock()

			if err := m.Queries.UpdateVolumeProgressStep(ctx, db.UpdateVolumeProgressStepParams{
				Step: step,
				ID:   vol.ID,
			}); err != nil {
				log.Printf("failed to update step for volume %s in job %s: %s", volumeName, jobID, err)
			}
			if err := m.Queries.UpdateVolumeProgressBytes(ctx, db.UpdateVolumeProgressBytesParams{
				TransferredBytes: transferred,
				TotalBytes:       total,
				ID:               vol.ID,
			}); err != nil {
				log.Printf("failed to update bytes for volume %s in job %s: %s", volumeName, jobID, err)
			}
		},
		OnBackupPath: func(volumeName, backupPath string) {
			m.mu.RLock()
			job, ok := m.jobs[jobID]
			m.mu.RUnlock()
			if !ok {
				return
			}

			var vol *VolumeRow
			for i := range job.Volumes {
				if job.Volumes[i].VolumeName == volumeName {
					vol = &job.Volumes[i]
					break
				}
			}
			if vol == nil {
				return
			}

			vol.mu.Lock()
			vol.BackupPath = backupPath
			vol.mu.Unlock()

			if err := m.Queries.UpdateVolumeProgressBackupPath(ctx, db.UpdateVolumeProgressBackupPathParams{
				BackupPath: sql.NullString{String: backupPath, Valid: backupPath != ""},
				ID:         vol.ID,
			}); err != nil {
				log.Printf("failed to persist backup path for volume %s in job %s: %s", volumeName, jobID, err)
			}
		},
	}

	s.docker = newDockerClient()

	var err error
	if appsErr != nil {
		err = fmt.Errorf("list applications: %w", appsErr)
	} else {
		err = s.MoveApplicationStorage(opts)
	}
	completedAt = time.Now()

	status := string(JobCompleted)
	if err != nil {
		status = string(JobFailed)
		log.Printf("migration job %s failed: %s", jobID, err)

		// mark all pending volumes as failed
		m.mu.RLock()
		job, _ := m.jobs[jobID]
		m.mu.RUnlock()
		if job != nil {
			for i := range job.Volumes {
				job.Volumes[i].mu.Lock()
				needsFail := job.Volumes[i].Step != StepCompleted
				if needsFail {
					job.Volumes[i].Step = StepFailed
					job.Volumes[i].Error = err.Error()
				}
				job.Volumes[i].mu.Unlock()

				if needsFail {
					if err := m.Queries.UpdateVolumeProgressError(ctx, db.UpdateVolumeProgressErrorParams{
						ErrorMessage: sql.NullString{String: err.Error(), Valid: true},
						ID:           job.Volumes[i].ID,
					}); err != nil {
						log.Printf("failed to set error for volume %s: %s", job.Volumes[i].VolumeName, err)
					}
				}
			}
		}
	}

	if err := m.Queries.UpdateMigrationJobStatus(ctx, db.UpdateMigrationJobStatusParams{
		Status: status,
		ID:     jobID,
	}); err != nil {
		log.Printf("failed to set job %s status to %s: %s", jobID, status, err)
	}

	m.mu.Lock()
	if j, ok := m.jobs[jobID]; ok {
		j.Status = MigrationJobStatus(status)
		j.CompletedAt = completedAt
		j.UpdatedAt = time.Now()
	}
	m.mu.Unlock()

	// Write history
	{
		totalDuration := completedAt.Sub(startedAt).Milliseconds()
		recordVolumes := make(map[string]VolumeRecord)

		// Gather per-volume data from the job
		m.mu.RLock()
		job, hasJob := m.jobs[jobID]
		m.mu.RUnlock()

		if hasJob {
			for i := range job.Volumes {
				v := &job.Volumes[i]
				volStatus := status
				volError := ""
				if v.Step == StepFailed {
					volStatus = string(JobFailed)
					volError = v.Error
				}
				sourceDrive := getDriveForSourcePath(v.SourcePath, drives)
				recordVolumes[v.VolumeName] = VolumeRecord{
					VolumeName:  v.VolumeName,
					SourcePath:  v.SourcePath,
					SourceDrive: sourceDrive,
					DestPath:    v.DestPath,
					DestDrive:   v.DestDrive,
					TotalBytes:  v.TotalBytes,
					DurationMs:  totalDuration,
					Status:      volStatus,
					Error:       volError,
				}
			}
		}

		if len(recordVolumes) > 0 {
			volList := make([]VolumeRecord, 0, len(recordVolumes))
			for _, vr := range recordVolumes {
				volList = append(volList, vr)
			}

			record := MigrationRecord{
				JobID:       jobID,
				AppName:     app.Name,
				Status:      status,
				StartedAt:   startedAt,
				CompletedAt: completedAt,
				Volumes:     volList,
			}
			WriteMigrationHistory(m.Queries, record)
		}
	}
}

func newDockerClient() *client.Client {
	docker, err := client.New(client.FromEnv)
	if err != nil {
		log.Printf("warning: cannot create docker client: %s", err)
		return nil
	}
	return docker
}
