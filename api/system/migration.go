package system

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"dokvol/api/internal/db"

	"github.com/google/uuid"
	"github.com/moby/moby/client"
)

type MigrationJobStatus string

const (
	JobPending   MigrationJobStatus = "pending"
	JobRunning   MigrationJobStatus = "running"
	JobCompleted MigrationJobStatus = "completed"
	JobFailed    MigrationJobStatus = "failed"
)

const (
	StepPending      = "pending"
	StepStopping     = "stopping"
	StepSyncing      = "syncing"
	StepVerifying    = "verifying"
	StepRelinking    = "relinking"
	StepStarting     = "starting"
	StepCompleted    = "completed"
	StepFailed       = "failed"
)

type VolumeRow struct {
	ID          int64
	VolumeName  string
	Step        string
	TotalBytes  int64
	Transferred int64
	SourcePath  string
	DestPath    string
	DestDrive   string
	Error       string
}

type Job struct {
	ID      string
	AppName string
	Status  MigrationJobStatus
	Volumes []VolumeRow
}

type ProgressFn func(VolumeName, Step string, TransferredBytes, TotalBytes int64)

type MigrationManager struct {
	db     *db.Queries
	mu     sync.RWMutex
	jobs   map[string]*Job
}

func NewMigrationManager(queries *db.Queries) *MigrationManager {
	return &MigrationManager{
		db:   queries,
		jobs: make(map[string]*Job),
	}
}

func (m *MigrationManager) StartJob(ctx context.Context, appName string, application Application, volumes []ApplicationVolumeOptions) (string, error) {
	jobID := uuid.New().String()

	_, err := m.db.CreateMigrationJob(ctx, db.CreateMigrationJobParams{
		ID:      jobID,
		AppName: appName,
		Status:  string(JobPending),
	})
	if err != nil {
		return "", fmt.Errorf("create job: %w", err)
	}

	job := &Job{
		ID:      jobID,
		AppName: appName,
		Status:  JobPending,
		Volumes: make([]VolumeRow, len(volumes)),
	}

	for i, vol := range volumes {
		destPath := filepath.Join(vol.DestinationDrive.Mountpoint, DOKVOL_FOLDER, appName, vol.VolumeDetail.Name)

		p, err := m.db.CreateVolumeProgress(ctx, db.CreateVolumeProgressParams{
			JobID:      jobID,
			VolumeName: vol.VolumeDetail.Name,
			SourcePath: vol.VolumeDetail.Source,
			DestPath:   destPath,
			DestDrive:  vol.DestinationDrive.Mountpoint,
			Step:       StepPending,
		})
		if err != nil {
			return "", fmt.Errorf("create progress: %w", err)
		}

		job.Volumes[i] = VolumeRow{
			ID:         p.ID,
			VolumeName: vol.VolumeDetail.Name,
			Step:       StepPending,
			SourcePath: vol.VolumeDetail.Source,
			DestPath:   destPath,
			DestDrive:  vol.DestinationDrive.Mountpoint,
		}
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
		return job, nil
	}

	dbJob, err := m.db.GetMigrationJob(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}

	progressRows, err := m.db.ListVolumeProgressByJob(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list progress: %w", err)
	}

	job = &Job{
		ID:      dbJob.ID,
		AppName: dbJob.AppName,
		Status:  MigrationJobStatus(dbJob.Status),
		Volumes: make([]VolumeRow, len(progressRows)),
	}

	for i, p := range progressRows {
		job.Volumes[i] = VolumeRow{
			ID:         p.ID,
			VolumeName: p.VolumeName,
			Step:       p.Step,
			TotalBytes: p.TotalBytes,
			Transferred: p.TransferredBytes,
			SourcePath: p.SourcePath,
			DestPath:   p.DestPath,
			DestDrive:  p.DestDrive,
			Error:      p.ErrorMessage.String,
		}
	}

	return job, nil
}

func (m *MigrationManager) ListJobs(ctx context.Context) ([]*Job, error) {
	dbJobs, err := m.db.ListMigrationJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}

	jobs := make([]*Job, len(dbJobs))
	for i, j := range dbJobs {
		job, err := m.GetJob(ctx, j.ID)
		if err != nil {
			return nil, err
		}
		jobs[i] = job
	}

	return jobs, nil
}

func (m *MigrationManager) runJob(ctx context.Context, jobID string, app Application, volumes []ApplicationVolumeOptions) {
	_ = m.db.UpdateMigrationJobStatus(ctx, db.UpdateMigrationJobStatusParams{
		Status: string(JobRunning),
		ID:     jobID,
	})

	m.mu.Lock()
	if j, ok := m.jobs[jobID]; ok {
		j.Status = JobRunning
	}
	m.mu.Unlock()

	s := System{
		Drives:       GetDrives(),
		Applications: GetApplicationsDetails(GetDrives()),
	}

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

			vol.Step = step
			vol.TotalBytes = total
			vol.Transferred = transferred

			_ = m.db.UpdateVolumeProgressStep(ctx, db.UpdateVolumeProgressStepParams{
				Step: step,
				ID:   vol.ID,
			})
			_ = m.db.UpdateVolumeProgressBytes(ctx, db.UpdateVolumeProgressBytesParams{
				TransferredBytes: transferred,
				TotalBytes:       total,
				ID:               vol.ID,
			})
		},
	}

	s.docker = newDockerClient()

	err := s.MoveApplicationStorage(opts)

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
				if job.Volumes[i].Step != StepCompleted {
					_ = m.db.UpdateVolumeProgressError(ctx, db.UpdateVolumeProgressErrorParams{
						ErrorMessage: err.Error(),
						ID:           job.Volumes[i].ID,
					})
				}
			}
		}
	}

	_ = m.db.UpdateMigrationJobStatus(ctx, db.UpdateMigrationJobStatusParams{
		Status: status,
		ID:     jobID,
	})

	m.mu.Lock()
	if j, ok := m.jobs[jobID]; ok {
		j.Status = MigrationJobStatus(status)
	}
	m.mu.Unlock()
}

func newDockerClient() *client.Client {
	docker, err := client.New(client.FromEnv)
	if err != nil {
		log.Printf("warning: cannot create docker client: %s", err)
		return nil
	}
	return docker
}
