package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"dokvol/api/system"
	"dokvol/api/system/backup"

	"github.com/gin-gonic/gin"
)

type createTargetRequest struct {
	Name     string          `json:"name"`
	Provider string          `json:"provider"`
	Config   json.RawMessage `json:"config"`
}

type updateTargetRequest struct {
	Name     string          `json:"name"`
	Provider string          `json:"provider"`
	Config   json.RawMessage `json:"config"`
}

type runBackupRequest struct {
	TargetID string `json:"target_id"`
	AppName  string `json:"app_name"`
}

type restoreBackupRequest struct {
	JobID          string `json:"job_id"`
	TargetID       string `json:"target_id"`
	AppName        string `json:"app_name"`
	DestMountpoint string `json:"dest_mountpoint,omitempty"`
}

type createScheduleRequest struct {
	TargetID  string `json:"target_id"`
	AppName   string `json:"app_name"`
	CronExpr  string `json:"cron_expr"`
	Retention int    `json:"retention"`
}

type updateScheduleRequest struct {
	TargetID  string `json:"target_id"`
	AppName   string `json:"app_name"`
	CronExpr  string `json:"cron_expr"`
	Retention int    `json:"retention"`
	Enabled   *bool  `json:"enabled"`
}

func ListBackupTargets(c *gin.Context) {
	targets, err := backup.ListTargets(BackupEngine.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}
	if targets == nil {
		targets = []backup.BackupTarget{}
	}

	type targetJSON struct {
		ID        string            `json:"id"`
		Name      string            `json:"name"`
		Provider  backup.ProviderType `json:"provider"`
		CreatedAt string            `json:"created_at"`
		UpdatedAt string            `json:"updated_at"`
	}

	result := make([]targetJSON, len(targets))
	for i, t := range targets {
		result[i] = targetJSON{
			ID:        t.ID,
			Name:      t.Name,
			Provider:  t.Provider,
			CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt: t.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	c.JSON(http.StatusOK, result)
}

func CreateBackupTarget(c *gin.Context) {
	var req createTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "invalid request body", nil))
		return
	}
	if req.Name == "" || req.Provider == "" || req.Config == nil {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "name, provider, and config are required", nil))
		return
	}

	provider := backup.ProviderType(req.Provider)
	switch provider {
	case backup.ProviderS3, backup.ProviderSFTP, backup.ProviderLocal:
	default:
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "invalid provider, must be s3, sftp, or local", nil))
		return
	}

	var config interface{}
	switch provider {
	case backup.ProviderS3:
		config = &backup.S3Config{}
	case backup.ProviderSFTP:
		config = &backup.SFTPConfig{}
	case backup.ProviderLocal:
		config = &backup.LocalConfig{}
	}
	if err := json.Unmarshal(req.Config, config); err != nil {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "invalid config for provider", nil))
		return
	}

	target, err := backup.CreateTarget(BackupEngine.DB, req.Name, provider, config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":        target.ID,
		"name":      target.Name,
		"provider":  target.Provider,
		"created_at": target.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func UpdateBackupTarget(c *gin.Context) {
	id := c.Param("id")

	var req updateTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "invalid request body", nil))
		return
	}
	if req.Name == "" || req.Provider == "" || req.Config == nil {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "name, provider, and config are required", nil))
		return
	}

	provider := backup.ProviderType(req.Provider)
	var config interface{}
	switch provider {
	case backup.ProviderS3:
		config = &backup.S3Config{}
	case backup.ProviderSFTP:
		config = &backup.SFTPConfig{}
	case backup.ProviderLocal:
		config = &backup.LocalConfig{}
	default:
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "invalid provider", nil))
		return
	}
	if err := json.Unmarshal(req.Config, config); err != nil {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "invalid config", nil))
		return
	}

	if err := backup.UpdateTarget(BackupEngine.DB, id, req.Name, provider, config); err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func DeleteBackupTarget(c *gin.Context) {
	id := c.Param("id")
	if err := backup.DeleteTarget(BackupEngine.DB, id); err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func TestBackupTarget(c *gin.Context) {
	id := c.Param("id")

	target, err := backup.GetTarget(BackupEngine.DB, id)
	if err != nil {
		c.JSON(http.StatusNotFound, system.NewAPIError("INTERNAL_ERROR", "target not found", nil))
		return
	}

	configJSON, err := system.DecryptConfig(target.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", "decrypt failed", nil))
		return
	}

	rcloneConfig, err := backup.BuildRcloneConfig(target.Provider, configJSON)
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	cmd := exec.CommandContext(c.Request.Context(), "rclone", "lsf",
		"--config", "/dev/stdin",
		"--max-depth", "1",
		"backup-target:",
	)
	cmd.Stdin = strings.NewReader(rcloneConfig)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("connection failed: %s", stderr.String()),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "connection successful",
	})
}

func RunBackup(c *gin.Context) {
	var req runBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "invalid request body", nil))
		return
	}
	if req.TargetID == "" || req.AppName == "" {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "target_id and app_name are required", nil))
		return
	}

	jobID, err := BackupEngine.RunBackup(req.AppName, req.TargetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"job_id": jobID,
		"status": "running",
	})
}

func ListBackupJobs(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	jobs, total, err := backup.ListBackupJobs(BackupEngine.DB, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	type jobJSON struct {
		ID           string `json:"id"`
		TargetID     string `json:"target_id"`
		AppName      string `json:"app_name"`
		Status       string `json:"status"`
		TotalBytes   int64  `json:"total_bytes"`
		DurationMs   int64  `json:"duration_ms"`
		ErrorMessage string `json:"error_message,omitempty"`
		StartedAt    string `json:"started_at"`
		CompletedAt  string `json:"completed_at"`
	}

	result := make([]jobJSON, len(jobs))
	for i, j := range jobs {
		result[i] = jobJSON{
			ID:           j.ID,
			TargetID:     j.TargetID,
			AppName:      j.AppName,
			Status:       j.Status,
			TotalBytes:   j.TotalBytes,
			DurationMs:   j.DurationMs,
			ErrorMessage: j.ErrorMessage,
			StartedAt:    j.StartedAt.Format("2006-01-02T15:04:05Z"),
			CompletedAt:  j.CompletedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"jobs":  result,
		"total": total,
	})
}

func GetBackupJob(c *gin.Context) {
	id := c.Param("id")

	job := BackupEngine.GetJob(id)
	if job == nil {
		volumes, err := backup.GetBackupJobVolumes(BackupEngine.DB, id)
		if err != nil {
			c.JSON(http.StatusNotFound, system.NewAPIError("INTERNAL_ERROR", "job not found", nil))
			return
		}

		var j struct {
			ID       string                    `json:"id"`
			Status   string                    `json:"status"`
			Volumes  []backup.BackupVolumeProgress `json:"volumes"`
		}
		j.ID = id
		j.Status = "completed"
		j.Volumes = volumes
		c.JSON(http.StatusOK, j)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       job.ID,
		"status":   job.Status,
		"volumes":  job.Volumes,
	})
}

func ListBackupsOnTarget(c *gin.Context) {
	targetID := c.Param("id")
	appName := c.Query("app")

	entries, err := backup.ListBackups(BackupEngine.DB, targetID, appName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	if entries == nil {
		entries = []backup.BackupListEntry{}
	}
	c.JSON(http.StatusOK, entries)
}

func RestoreBackup(c *gin.Context) {
	var req restoreBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "invalid request body", nil))
		return
	}
	if req.JobID == "" || req.TargetID == "" || req.AppName == "" {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "job_id, target_id, and app_name are required", nil))
		return
	}

	result, err := backup.RestoreBackup(BackupEngine.DB, backup.RestoreOptions{
		JobID:          req.JobID,
		AppName:        req.AppName,
		TargetID:       req.TargetID,
		DestMountpoint: req.DestMountpoint,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, result)
}

func ListBackupSchedules(c *gin.Context) {
	rows, err := BackupEngine.DB.Query(
		`SELECT id, target_id, app_name, cron_expr, retention, enabled, created_at, updated_at FROM backup_schedule ORDER BY created_at`,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}
	defer rows.Close()

	type scheduleJSON struct {
		ID        string `json:"id"`
		TargetID  string `json:"target_id"`
		AppName   string `json:"app_name"`
		CronExpr  string `json:"cron_expr"`
		Retention int    `json:"retention"`
		Enabled   bool   `json:"enabled"`
	}

	var result []scheduleJSON
	for rows.Next() {
		var s scheduleJSON
		var enabled int
		var createdAt, updatedAt string
		if err := rows.Scan(&s.ID, &s.TargetID, &s.AppName, &s.CronExpr, &s.Retention, &enabled, &createdAt, &updatedAt); err != nil {
			continue
		}
		s.Enabled = enabled == 1
		result = append(result, s)
	}

	if result == nil {
		result = []scheduleJSON{}
	}
	c.JSON(http.StatusOK, result)
}

func CreateBackupSchedule(c *gin.Context) {
	var req createScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "invalid request body", nil))
		return
	}
	if req.TargetID == "" || req.AppName == "" || req.CronExpr == "" {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "target_id, app_name, and cron_expr are required", nil))
		return
	}
	if req.Retention <= 0 {
		req.Retention = 7
	}

	schedule, err := backup.CreateSchedule(BackupEngine.DB, req.TargetID, req.AppName, req.CronExpr, req.Retention)
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id": schedule.ID,
	})
}

func UpdateBackupSchedule(c *gin.Context) {
	id := c.Param("id")
	var req updateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "invalid request body", nil))
		return
	}

	if err := backup.UpdateSchedule(BackupEngine.DB, id, req.AppName, req.CronExpr, req.Retention, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func DeleteBackupSchedule(c *gin.Context) {
	id := c.Param("id")
	if err := backup.DeleteSchedule(BackupEngine.DB, id); err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
