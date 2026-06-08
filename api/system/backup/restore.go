package backup

import (
	"bytes"
	"database/sql"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"dokvol/api/system"
)

type RestoreOptions struct {
	JobID       string
	AppName     string
	TargetID    string
	DestMountpoint string
}

type RestoreResult struct {
	JobID      string   `json:"job_id"`
	AppName    string   `json:"app_name"`
	Status     string   `json:"status"`
	Volumes    []RestoreVolumeResult `json:"volumes"`
}

type RestoreVolumeResult struct {
	VolumeName string `json:"volume_name"`
	DestPath   string `json:"dest_path"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

func RestoreBackup(db *sql.DB, opts RestoreOptions) (*RestoreResult, error) {
	target, err := GetTarget(db, opts.TargetID)
	if err != nil {
		return nil, fmt.Errorf("get target: %w", err)
	}

	configJSON, err := system.DecryptConfig(target.Config)
	if err != nil {
		return nil, fmt.Errorf("decrypt config: %w", err)
	}

	rows, err := db.Query(
		`SELECT volume_name, source_path, backup_path FROM backup_job_volume WHERE job_id = ? ORDER BY id`,
		opts.JobID,
	)
	if err != nil {
		return nil, fmt.Errorf("list backup volumes: %w", err)
	}
	defer rows.Close()

	rcloneConfig, err := BuildRcloneConfig(target.Provider, configJSON)
	if err != nil {
		return nil, fmt.Errorf("build rclone config: %w", err)
	}

	result := &RestoreResult{
		JobID:   opts.JobID,
		AppName: opts.AppName,
		Status:  "completed",
	}

	for rows.Next() {
		var volName, sourcePath, backupPath string
		if err := rows.Scan(&volName, &sourcePath, &backupPath); err != nil {
			return nil, fmt.Errorf("scan volume: %w", err)
		}

		destPath := sourcePath
		if opts.DestMountpoint != "" {
			destPath = strings.Replace(sourcePath, sourcePath, opts.DestMountpoint+"/"+volName, 1)
		}

		cmd := exec.Command("rclone", "copy",
			"--config", "/dev/stdin",
			"--progress",
			"--checksum",
			"--verbose",
			"backup-target:"+backupPath+"/",
			destPath+"/",
		)
		cmd.Stdin = strings.NewReader(rcloneConfig)

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		volResult := RestoreVolumeResult{
			VolumeName: volName,
			DestPath:   destPath,
			Status:     "completed",
		}

		if err := cmd.Run(); err != nil {
			volResult.Status = "failed"
			volResult.Error = fmt.Sprintf("rclone restore failed: %s\n%s", err, stderr.String())
			result.Status = "completed_with_errors"
		}

		result.Volumes = append(result.Volumes, volResult)
	}

	_, _ = db.Exec(
		`UPDATE backup_job SET status = ?, completed_at = ? WHERE id = ?`,
		result.Status, time.Now(), opts.JobID,
	)

	return result, nil
}
