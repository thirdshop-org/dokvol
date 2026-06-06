package system

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dokvol/api/internal/db"
)

const HISTORY_FOLDER = "history"

type VolumeRecord struct {
	VolumeName  string `json:"volume_name"`
	SourcePath  string `json:"source_path"`
	SourceDrive string `json:"source_drive,omitempty"`
	DestPath    string `json:"dest_path"`
	DestDrive   string `json:"dest_drive"`
	TotalBytes  int64  `json:"total_bytes"`
	DurationMs  int64  `json:"duration_ms"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

type MigrationRecord struct {
	JobID       string          `json:"job_id"`
	AppName     string          `json:"app_name"`
	Status      string          `json:"status"`
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt time.Time       `json:"completed_at"`
	Volumes     []VolumeRecord  `json:"volumes"`
}

func getDriveForSourcePath(path string, drives []DriveInfo) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	var best string
	bestLen := 0
	for _, d := range drives {
		if strings.HasPrefix(path, d.Mountpoint) && len(d.Mountpoint) > bestLen {
			best = d.Mountpoint
			bestLen = len(d.Mountpoint)
		}
	}
	return best
}

func uniqueDrives(volumes []VolumeRecord) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range volumes {
		for _, drive := range []string{v.SourceDrive, v.DestDrive} {
			if drive != "" && !seen[drive] {
				seen[drive] = true
				result = append(result, drive)
			}
		}
	}
	return result
}

func historyDir(driveMountpoint string) string {
	return filepath.Join(driveMountpoint, DOKVOL_FOLDER, HISTORY_FOLDER)
}

func WriteMigrationHistory(queries *db.Queries, record MigrationRecord) error {
	drives := uniqueDrives(record.Volumes)

	for _, drive := range drives {
		dir := historyDir(drive)
		if err := os.MkdirAll(dir, 0700); err != nil {
			log.Printf("failed to create history dir %s: %s", dir, err)
			continue
		}

		filePath := filepath.Join(dir, record.JobID+".json")
		data, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			log.Printf("failed to marshal history for %s: %s", record.JobID, err)
			continue
		}

		if err := os.WriteFile(filePath, data, 0600); err != nil {
			log.Printf("failed to write history file %s: %s", filePath, err)
		}
	}

	for _, vol := range record.Volumes {
		var startedAt, completedAt sql.NullTime
		if !record.StartedAt.IsZero() {
			startedAt = sql.NullTime{Time: record.StartedAt, Valid: true}
		}
		if !record.CompletedAt.IsZero() {
			completedAt = sql.NullTime{Time: record.CompletedAt, Valid: true}
		}

		var srcDrive sql.NullString
		if vol.SourceDrive != "" {
			srcDrive = sql.NullString{String: vol.SourceDrive, Valid: true}
		}

		var errMsg sql.NullString
		if vol.Error != "" {
			errMsg = sql.NullString{String: vol.Error, Valid: true}
		}

		_, err := queries.CreateMigrationLog(context.Background(), db.CreateMigrationLogParams{
			JobID:        record.JobID,
			AppName:      record.AppName,
			VolumeName:   vol.VolumeName,
			SourcePath:   vol.SourcePath,
			SourceDrive:  srcDrive,
			DestPath:     vol.DestPath,
			DestDrive:    vol.DestDrive,
			TotalBytes:   vol.TotalBytes,
			DurationMs:   vol.DurationMs,
			Status:       vol.Status,
			ErrorMessage: errMsg,
			StartedAt:    startedAt,
			CompletedAt:  completedAt,
		})
		if err != nil {
			log.Printf("failed to insert migration_log for %s/%s: %s", record.JobID, vol.VolumeName, err)
		}
	}

	return nil
}

func ScanDriveHistory(queries *db.Queries, drives []DriveInfo) error {
	for _, d := range drives {
		dir := historyDir(d.Mountpoint)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			log.Printf("failed to read history dir %s: %s", dir, err)
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}

			filePath := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				log.Printf("failed to read history file %s: %s", filePath, err)
				continue
			}

			var record MigrationRecord
			if err := json.Unmarshal(data, &record); err != nil {
				log.Printf("failed to parse history file %s: %s", filePath, err)
				continue
			}

			if len(record.Volumes) == 0 {
				continue
			}

			existing, err := queries.GetMigrationLogByJobID(context.Background(), record.JobID)
			if err != nil {
				log.Printf("failed to check existing log for %s: %s", record.JobID, err)
				continue
			}
			if len(existing) > 0 {
				continue
			}

			for _, vol := range record.Volumes {
				var startedAt, completedAt sql.NullTime
				if !record.StartedAt.IsZero() {
					startedAt = sql.NullTime{Time: record.StartedAt, Valid: true}
				}
				if !record.CompletedAt.IsZero() {
					completedAt = sql.NullTime{Time: record.CompletedAt, Valid: true}
				}
				var srcDrive sql.NullString
				if vol.SourceDrive != "" {
					srcDrive = sql.NullString{String: vol.SourceDrive, Valid: true}
				}
				var errMsg sql.NullString
				if vol.Error != "" {
					errMsg = sql.NullString{String: vol.Error, Valid: true}
				}

				if _, err := queries.CreateMigrationLog(context.Background(), db.CreateMigrationLogParams{
					JobID:        record.JobID,
					AppName:      record.AppName,
					VolumeName:   vol.VolumeName,
					SourcePath:   vol.SourcePath,
					SourceDrive:  srcDrive,
					DestPath:     vol.DestPath,
					DestDrive:    vol.DestDrive,
					TotalBytes:   vol.TotalBytes,
					DurationMs:   vol.DurationMs,
					Status:       vol.Status,
					ErrorMessage: errMsg,
					StartedAt:    startedAt,
					CompletedAt:  completedAt,
				}); err != nil {
					log.Printf("failed to insert scanned migration_log for %s/%s: %s", record.JobID, vol.VolumeName, err)
				}
			}
		}
	}
	return nil
}
