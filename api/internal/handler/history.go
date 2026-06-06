package handler

import (
	"net/http"
	"strconv"

	"dokvol/api/internal/db"
	"dokvol/api/system"

	"github.com/gin-gonic/gin"
)

type historyEntryJSON struct {
	ID           int64  `json:"id"`
	JobID        string `json:"job_id"`
	AppName      string `json:"app_name"`
	VolumeName   string `json:"volume_name"`
	SourcePath   string `json:"source_path"`
	SourceDrive  string `json:"source_drive,omitempty"`
	DestPath     string `json:"dest_path"`
	DestDrive    string `json:"dest_drive"`
	TotalBytes   int64  `json:"total_bytes"`
	DurationMs   int64  `json:"duration_ms"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message,omitempty"`
	StartedAt    string `json:"started_at,omitempty"`
	CompletedAt  string `json:"completed_at,omitempty"`
	CreatedAt    string `json:"created_at"`
}

type historyListResponse struct {
	Entries []historyEntryJSON `json:"entries"`
	Total   int64              `json:"total"`
}

func rowToHistoryJSON(r db.MigrationLog) historyEntryJSON {
	h := historyEntryJSON{
		ID:          r.ID,
		JobID:       r.JobID,
		AppName:     r.AppName,
		VolumeName:  r.VolumeName,
		SourcePath:  r.SourcePath,
		DestPath:    r.DestPath,
		DestDrive:   r.DestDrive,
		TotalBytes:  r.TotalBytes,
		DurationMs:  r.DurationMs,
		Status:      r.Status,
		CreatedAt:   r.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
	if r.SourceDrive.Valid {
		h.SourceDrive = r.SourceDrive.String
	}
	if r.ErrorMessage.Valid {
		h.ErrorMessage = r.ErrorMessage.String
	}
	if r.StartedAt.Valid {
		h.StartedAt = r.StartedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	if r.CompletedAt.Valid {
		h.CompletedAt = r.CompletedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	return h
}

func ListHistory(c *gin.Context) {
	limit := int64(50)
	offset := int64(0)

	if l, err := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64); err == nil && l > 0 && l <= 200 {
		limit = l
	}
	if o, err := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64); err == nil && o >= 0 {
		offset = o
	}

	var rows []db.MigrationLog
	var err error

	switch {
	case c.Query("app") != "":
		rows, err = MigrationManager.Queries.ListMigrationLogsByApp(c.Request.Context(), db.ListMigrationLogsByAppParams{
			AppName: c.Query("app"),
			Limit:   limit,
			Offset:  offset,
		})
	case c.Query("drive") != "":
		rows, err = MigrationManager.Queries.ListMigrationLogsByDrive(c.Request.Context(), db.ListMigrationLogsByDriveParams{
			DestDrive: c.Query("drive"),
			Limit:     limit,
			Offset:    offset,
		})
	case c.Query("status") != "":
		rows, err = MigrationManager.Queries.ListMigrationLogsByStatus(c.Request.Context(), db.ListMigrationLogsByStatusParams{
			Status: c.Query("status"),
			Limit:  limit,
			Offset: offset,
		})
	default:
		rows, err = MigrationManager.Queries.ListMigrationLogs(c.Request.Context(), db.ListMigrationLogsParams{
			Limit:  limit,
			Offset: offset,
		})
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	total, err := MigrationManager.Queries.CountMigrationLogs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	entries := make([]historyEntryJSON, len(rows))
	for i, r := range rows {
		entries[i] = rowToHistoryJSON(r)
	}

	c.JSON(http.StatusOK, historyListResponse{
		Entries: entries,
		Total:   total,
	})
}

type historyJobJSON struct {
	JobID      string             `json:"job_id"`
	AppName    string             `json:"app_name"`
	Status     string             `json:"status"`
	StartedAt  string             `json:"started_at,omitempty"`
	CompletedAt string            `json:"completed_at,omitempty"`
	Volumes    []historyEntryJSON `json:"volumes"`
}

func GetHistoryJob(c *gin.Context) {
	id := c.Param("id")

	rows, err := MigrationManager.Queries.GetMigrationLogByJobID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}
	if len(rows) == 0 {
		c.JSON(http.StatusNotFound, system.NewAPIError("NOT_FOUND", "job not found", nil))
		return
	}

	volumes := make([]historyEntryJSON, len(rows))
	for i, r := range rows {
		volumes[i] = rowToHistoryJSON(r)
	}

	resp := historyJobJSON{
		JobID:      rows[0].JobID,
		AppName:    rows[0].AppName,
		Status:     rows[0].Status,
		Volumes:    volumes,
	}
	if rows[0].StartedAt.Valid {
		resp.StartedAt = rows[0].StartedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	if rows[0].CompletedAt.Valid {
		resp.CompletedAt = rows[0].CompletedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}

	c.JSON(http.StatusOK, resp)
}

func RescanHistory(c *gin.Context) {
	drives := system.GetDrives()
	if err := system.ScanDriveHistory(MigrationManager.Queries, drives); err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "history rescanned"})
}
