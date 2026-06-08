package backup

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"dokvol/api/system"

	"github.com/google/uuid"
)

type VolumeBackupStatus string

const (
	VolumePending   VolumeBackupStatus = "pending"
	VolumeBackingUp VolumeBackupStatus = "backing_up"
	VolumeCompleted VolumeBackupStatus = "completed"
	VolumeFailed    VolumeBackupStatus = "failed"
)

type BackupJobProgress struct {
	ID        string
	TargetID  string
	AppName   string
	Status    string
	StartedAt time.Time
	Volumes   []BackupVolumeProgress
	Error     string
}

type BackupVolumeProgress struct {
	VolumeName       string
	SourcePath       string
	BackupPath       string
	Status           VolumeBackupStatus
	TotalBytes       int64
	TransferredBytes int64
	ErrorMessage     string
}

type BackupEngine struct {
	DB   *sql.DB
	mu   sync.RWMutex
	jobs map[string]*BackupJobProgress
}

func NewBackupEngine(database *sql.DB) *BackupEngine {
	return &BackupEngine{
		DB:   database,
		jobs: make(map[string]*BackupJobProgress),
	}
}

func volumeSubdir(v system.VolumeDetail) string {
	if v.Name != "" {
		return v.Name
	}
	return strings.TrimLeft(v.Destination, "/")
}

func (e *BackupEngine) RunBackup(appName string, targetID string) (string, error) {
	jobID := uuid.New().String()
	now := time.Now()

	target, err := GetTarget(e.DB, targetID)
	if err != nil {
		return "", fmt.Errorf("get target: %w", err)
	}

	configJSON, err := system.DecryptConfig(target.Config)
	if err != nil {
		return "", fmt.Errorf("decrypt target config: %w", err)
	}

	drives := system.GetDrives()
	apps := system.GetApplicationsDetails(drives)

	var app *system.Application
	for i := range apps {
		if apps[i].Name == appName || apps[i].Name == "/"+appName {
			app = &apps[i]
			break
		}
	}
	if app == nil {
		return "", fmt.Errorf("application '%s' not found", appName)
	}

	backupPathPrefix := fmt.Sprintf("dokvol-backups/%s/%s", strings.TrimLeft(appName, "/"), time.Now().Format("2006-01-02_150405"))

	_, err = e.DB.Exec(
		`INSERT INTO backup_job (id, target_id, app_name, status, started_at, created_at) VALUES (?, ?, ?, 'running', ?, ?)`,
		jobID, targetID, appName, now, now,
	)
	if err != nil {
		return "", fmt.Errorf("insert backup job: %w", err)
	}

	var volumes []BackupVolumeProgress
	for _, vol := range app.DockerVolumes {
		if !vol.IsMigratable {
			continue
		}
		volName := volumeSubdir(vol)
		backupPath := backupPathPrefix + "/" + volName

		_, err := e.DB.Exec(
			`INSERT INTO backup_job_volume (job_id, volume_name, source_path, backup_path, status) VALUES (?, ?, ?, ?, 'pending')`,
			jobID, volName, vol.Source, backupPath,
		)
		if err != nil {
			return "", fmt.Errorf("insert backup volume: %w", err)
		}

		volumes = append(volumes, BackupVolumeProgress{
			VolumeName: volName,
			SourcePath: vol.Source,
			BackupPath: backupPath,
			Status:     VolumePending,
		})
	}

	job := &BackupJobProgress{
		ID:        jobID,
		TargetID:  targetID,
		AppName:   appName,
		Status:    "running",
		StartedAt: now,
		Volumes:   volumes,
	}

	e.mu.Lock()
	e.jobs[jobID] = job
	e.mu.Unlock()

	go e.runBackupJob(jobID, target, configJSON, app, volumes)

	return jobID, nil
}

func (e *BackupEngine) runBackupJob(jobID string, target *BackupTarget, configJSON string, app *system.Application, volumes []BackupVolumeProgress) {
	startedAt := time.Now()
	var totalBytes int64
	allCompleted := true

	for i := range volumes {
		vol := &volumes[i]

		e.mu.Lock()
		if j, ok := e.jobs[jobID]; ok {
			j.Volumes[i].Status = VolumeBackingUp
		}
		e.mu.Unlock()

		e.updateVolumeStatus(jobID, vol.VolumeName, "backing_up")

		rcloneConfig, err := BuildRcloneConfig(target.Provider, configJSON)
		if err != nil {
			e.failVolume(jobID, vol.VolumeName, fmt.Sprintf("build config: %s", err))
			allCompleted = false
			continue
		}

		destPath := vol.BackupPath
		sourcePath := vol.SourcePath

		cmd := exec.Command("rclone", "copy",
			"--config", "/dev/stdin",
			"--progress",
			"--checksum",
			"--verbose",
			sourcePath+"/",
			"backup-target:"+destPath+"/",
		)
		cmd.Stdin = strings.NewReader(rcloneConfig)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			e.failVolume(jobID, vol.VolumeName, fmt.Sprintf("rclone failed: %s\n%s", err, stderr.String()))
			allCompleted = false
			continue
		}

		var size int64
		sizeCmd := exec.Command("rclone", "size",
			"--config", "/dev/stdin",
			"--json",
			"backup-target:"+destPath+"/",
		)
		sizeCmd.Stdin = strings.NewReader(rcloneConfig)
		sizeOut, err := sizeCmd.Output()
		if err == nil {
			var sizeResult struct {
				Count int64 `json:"count"`
				Bytes int64 `json:"bytes"`
			}
			if json.Unmarshal(sizeOut, &sizeResult) == nil {
				size = sizeResult.Bytes
			}
		}

		totalBytes += size

		e.mu.Lock()
		if j, ok := e.jobs[jobID]; ok {
			j.Volumes[i].Status = VolumeCompleted
			j.Volumes[i].TotalBytes = size
			j.Volumes[i].TransferredBytes = size
		}
		e.mu.Unlock()

		e.updateVolumeSuccess(jobID, vol.VolumeName, size)
	}

	completedAt := time.Now()
	durationMs := completedAt.Sub(startedAt).Milliseconds()
	finalStatus := "completed"
	var errMsg string
	if !allCompleted {
		finalStatus = "completed_with_errors"
		errMsg = "some volumes failed"
	}

	e.mu.Lock()
	if j, ok := e.jobs[jobID]; ok {
		j.Status = finalStatus
	}
	e.mu.Unlock()

	var errMsgSQL interface{}
	if errMsg != "" {
		errMsgSQL = errMsg
	}
	_, _ = e.DB.Exec(
		`UPDATE backup_job SET status = ?, total_bytes = ?, duration_ms = ?, completed_at = ?, error_message = ? WHERE id = ?`,
		finalStatus, totalBytes, durationMs, completedAt, errMsgSQL, jobID,
	)
}

func BuildRcloneConfig(provider ProviderType, configJSON string) (string, error) {
	switch provider {
	case ProviderS3:
		var cfg S3Config
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return "", fmt.Errorf("parse s3 config: %w", err)
		}
		return fmt.Sprintf(`[backup-target]
type = s3
provider = Other
env_auth = false
access_key_id = %s
secret_access_key = %s
endpoint = %s
region = %s
bucket = %s
force_path_style = %v
`, cfg.AccessKey, cfg.SecretKey, cfg.Endpoint, cfg.Region, cfg.Bucket, cfg.PathStyle), nil

	case ProviderSFTP:
		var cfg SFTPConfig
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return "", fmt.Errorf("parse sftp config: %w", err)
		}
		return fmt.Sprintf(`[backup-target]
type = sftp
host = %s
port = %d
user = %s
pass = %s
key_file = %s
`, cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.KeyPath), nil

	case ProviderLocal:
		var cfg LocalConfig
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return "", fmt.Errorf("parse local config: %w", err)
		}
		return `[backup-target]
type = local
`, nil
	}

	return "", fmt.Errorf("unsupported provider: %s", provider)
}

func (e *BackupEngine) updateVolumeStatus(jobID, volumeName, status string) {
	_, _ = e.DB.Exec(
		`UPDATE backup_job_volume SET status = ?, updated_at = ? WHERE job_id = ? AND volume_name = ?`,
		status, time.Now(), jobID, volumeName,
	)
}

func (e *BackupEngine) updateVolumeSuccess(jobID, volumeName string, bytes int64) {
	_, _ = e.DB.Exec(
		`UPDATE backup_job_volume SET status = 'completed', total_bytes = ?, transferred_bytes = ?, updated_at = ? WHERE job_id = ? AND volume_name = ?`,
		bytes, bytes, time.Now(), jobID, volumeName,
	)
}

func (e *BackupEngine) failVolume(jobID, volumeName, errMsg string) {
	e.mu.Lock()
	if j, ok := e.jobs[jobID]; ok {
		for i := range j.Volumes {
			if j.Volumes[i].VolumeName == volumeName {
				j.Volumes[i].Status = VolumeFailed
				j.Volumes[i].ErrorMessage = errMsg
				break
			}
		}
	}
	e.mu.Unlock()

	_, _ = e.DB.Exec(
		`UPDATE backup_job_volume SET status = 'failed', error_message = ?, updated_at = ? WHERE job_id = ? AND volume_name = ?`,
		errMsg, time.Now(), jobID, volumeName,
	)
}

func (e *BackupEngine) GetJob(jobID string) *BackupJobProgress {
	e.mu.RLock()
	defer e.mu.RUnlock()
	job, ok := e.jobs[jobID]
	if !ok {
		return nil
	}
	return job
}

func (e *BackupEngine) ListJobs() []*BackupJobProgress {
	e.mu.RLock()
	defer e.mu.RUnlock()
	jobs := make([]*BackupJobProgress, 0, len(e.jobs))
	for _, j := range e.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}
