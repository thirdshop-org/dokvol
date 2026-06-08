package backup

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"dokvol/api/system"
)

type BackupListEntry struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

func ListBackups(db *sql.DB, targetID, appName string) ([]BackupListEntry, error) {
	target, err := GetTarget(db, targetID)
	if err != nil {
		return nil, fmt.Errorf("get target: %w", err)
	}

	configJSON, err := system.DecryptConfig(target.Config)
	if err != nil {
		return nil, fmt.Errorf("decrypt config: %w", err)
	}

	rcloneConfig, err := BuildRcloneConfig(target.Provider, configJSON)
	if err != nil {
		return nil, fmt.Errorf("build rclone config: %w", err)
	}

	remotePath := fmt.Sprintf("backup-target:dokvol-backups/%s", strings.TrimLeft(appName, "/"))

	cmd := exec.Command("rclone", "lsjson",
		"--config", "/dev/stdin",
		"--recursive",
		"--dirs-only",
		remotePath,
	)
	cmd.Stdin = strings.NewReader(rcloneConfig)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("rclone lsjson failed: %s\n%s", err, stderr.String())
	}

	var entries []struct {
		Path     string `json:"Path"`
		Name     string `json:"Name"`
		Size     int64  `json:"Size"`
		ModTime  string `json:"ModTime"`
		IsDir    bool   `json:"IsDir"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &entries); err != nil {
		return nil, fmt.Errorf("parse rclone output: %w", err)
	}

	var backups []BackupListEntry
	for _, e := range entries {
		if !e.IsDir {
			continue
		}
		t, _ := time.Parse(time.RFC3339, e.ModTime)
		backups = append(backups, BackupListEntry{
			Path:       e.Path,
			Name:       e.Name,
			ModifiedAt: t,
		})
	}

	return backups, nil
}

func ListBackupJobsByApp(db *sql.DB, appName string) ([]BackupJob, error) {
	rows, err := db.Query(
		`SELECT id, target_id, app_name, status, total_bytes, duration_ms, COALESCE(error_message,''), started_at, completed_at, created_at
		 FROM backup_job WHERE app_name = ? ORDER BY created_at DESC`,
		appName,
	)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []BackupJob
	for rows.Next() {
		var j BackupJob
		var startedAt, completedAt, createdAt sql.NullTime
		var errMsg sql.NullString
		if err := rows.Scan(&j.ID, &j.TargetID, &j.AppName, &j.Status, &j.TotalBytes, &j.DurationMs, &errMsg, &startedAt, &completedAt, &createdAt); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		if startedAt.Valid { j.StartedAt = startedAt.Time }
		if completedAt.Valid { j.CompletedAt = completedAt.Time }
		if createdAt.Valid { j.CreatedAt = createdAt.Time }
		if errMsg.Valid { j.ErrorMessage = errMsg.String }
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func ListBackupJobs(db *sql.DB, limit, offset int) ([]BackupJob, int, error) {
	var total int
	_ = db.QueryRow(`SELECT COUNT(*) FROM backup_job`).Scan(&total)

	rows, err := db.Query(
		`SELECT id, target_id, app_name, status, total_bytes, duration_ms, COALESCE(error_message,''), started_at, completed_at, created_at
		 FROM backup_job ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []BackupJob
	for rows.Next() {
		var j BackupJob
		var startedAt, completedAt, createdAt sql.NullTime
		var errMsg sql.NullString
		if err := rows.Scan(&j.ID, &j.TargetID, &j.AppName, &j.Status, &j.TotalBytes, &j.DurationMs, &errMsg, &startedAt, &completedAt, &createdAt); err != nil {
			return nil, 0, fmt.Errorf("scan job: %w", err)
		}
		if startedAt.Valid { j.StartedAt = startedAt.Time }
		if completedAt.Valid { j.CompletedAt = completedAt.Time }
		if createdAt.Valid { j.CreatedAt = createdAt.Time }
		if errMsg.Valid { j.ErrorMessage = errMsg.String }
		jobs = append(jobs, j)
	}
	return jobs, total, nil
}

func GetBackupJobVolumes(db *sql.DB, jobID string) ([]BackupVolumeProgress, error) {
	rows, err := db.Query(
		`SELECT volume_name, source_path, backup_path, status, total_bytes, transferred_bytes, COALESCE(error_message,'')
		 FROM backup_job_volume WHERE job_id = ? ORDER BY id`,
		jobID,
	)
	if err != nil {
		return nil, fmt.Errorf("list volumes: %w", err)
	}
	defer rows.Close()

	var vols []BackupVolumeProgress
	for rows.Next() {
		var v BackupVolumeProgress
		var errMsg sql.NullString
		if err := rows.Scan(&v.VolumeName, &v.SourcePath, &v.BackupPath, &v.Status, &v.TotalBytes, &v.TransferredBytes, &errMsg); err != nil {
			return nil, fmt.Errorf("scan volume: %w", err)
		}
		if errMsg.Valid { v.ErrorMessage = errMsg.String }
		vols = append(vols, v)
	}
	return vols, nil
}
