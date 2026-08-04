package handler

import (
	"net/http"
	"strconv"

	"dokvol/api/system"

	"github.com/gin-gonic/gin"
)

type trashEntryJSON struct {
	ID         int64  `json:"id"`
	JobID      string `json:"job_id"`
	AppName    string `json:"app_name"`
	VolumeName string `json:"volume_name"`
	SourcePath string `json:"source_path"`
	DestPath   string `json:"dest_path"`
	DestDrive  string `json:"dest_drive"`
	Step       string `json:"step"`
	BackupPath string `json:"backup_path"`
}

// ListTrash returns every migrated (or interrupted) volume whose
// pre-migration backup has not yet been reclaimed.
func ListTrash(c *gin.Context) {
	entries, err := system.ListTrash(DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError(
			"INTERNAL_ERROR",
			err.Error(),
			nil,
		))
		return
	}

	result := make([]trashEntryJSON, len(entries))
	for i, e := range entries {
		result[i] = trashEntryJSON{
			ID:         e.VolumeProgressID,
			JobID:      e.JobID,
			AppName:    e.AppName,
			VolumeName: e.VolumeName,
			SourcePath: e.SourcePath,
			DestPath:   e.DestPath,
			DestDrive:  e.DestDrive,
			Step:       e.Step,
			BackupPath: e.BackupPath,
		}
	}
	c.JSON(http.StatusOK, result)
}

func trashEntryID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, system.NewAPIError(
			"INTERNAL_ERROR",
			"invalid trash entry id",
			nil,
		))
		return 0, false
	}
	return id, true
}

// RestoreTrash undoes a migration: stops whatever containers are using the
// volume, swaps the symlink back for the pre-migration data, and restarts
// them.
func RestoreTrash(c *gin.Context) {
	id, ok := trashEntryID(c)
	if !ok {
		return
	}

	if err := system.RestoreTrashEntry(DB, id); err != nil {
		if apiErr, ok := err.(*system.APIError); ok {
			c.JSON(apiErr.HTTPStatus(), apiErr)
			return
		}
		c.JSON(http.StatusInternalServerError, system.NewAPIError(
			"INTERNAL_ERROR",
			err.Error(),
			nil,
		))
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "restored"})
}

// PurgeTrash permanently deletes a pre-migration backup, freeing the disk.
// The already-migrated data is untouched.
func PurgeTrash(c *gin.Context) {
	id, ok := trashEntryID(c)
	if !ok {
		return
	}

	if err := system.PurgeTrashEntry(DB, id); err != nil {
		if apiErr, ok := err.(*system.APIError); ok {
			c.JSON(apiErr.HTTPStatus(), apiErr)
			return
		}
		c.JSON(http.StatusInternalServerError, system.NewAPIError(
			"INTERNAL_ERROR",
			err.Error(),
			nil,
		))
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "purged"})
}
