package handler

import (
	"database/sql"
	"net/http"
	"time"

	"dokvol/api/internal/db"
	"dokvol/api/system"

	"github.com/gin-gonic/gin"
)

type statsQuery struct {
	Name string `form:"name"`
	From string `form:"from"`
	To   string `form:"to"`
}

func parseTimeRange(c *gin.Context) (time.Time, time.Time, bool) {
	var q statsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "invalid query", nil))
		return time.Time{}, time.Time{}, false
	}

	from := time.Now().AddDate(0, 0, -7)
	to := time.Now()

	if q.From != "" {
		if t, err := time.Parse(time.RFC3339, q.From); err == nil {
			from = t
		}
	}
	if q.To != "" {
		if t, err := time.Parse(time.RFC3339, q.To); err == nil {
			to = t
		}
	}

	return from, to, true
}

func ListStatsVolume(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "'name' query parameter required", nil))
		return
	}

	from, to, ok := parseTimeRange(c)
	if !ok {
		return
	}

	rows, err := MigrationManager.Queries.ListStatsVolumeByName(c.Request.Context(), db.ListStatsVolumeByNameParams{
		VolumeName:  name,
		CapturedAt:  sql.NullTime{Time: from, Valid: true},
		CapturedAt_2: sql.NullTime{Time: to, Valid: true},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, rows)
}

func ListStatsDrive(c *gin.Context) {
	mountpoint := c.Query("mountpoint")
	if mountpoint == "" {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "'mountpoint' query parameter required", nil))
		return
	}

	from, to, ok := parseTimeRange(c)
	if !ok {
		return
	}

	rows, err := MigrationManager.Queries.ListStatsDriveByMountpoint(c.Request.Context(), db.ListStatsDriveByMountpointParams{
		Mountpoint:  mountpoint,
		CapturedAt:  sql.NullTime{Time: from, Valid: true},
		CapturedAt_2: sql.NullTime{Time: to, Valid: true},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, rows)
}

func ListStatsApplication(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, system.NewAPIError("INTERNAL_ERROR", "'name' query parameter required", nil))
		return
	}

	from, to, ok := parseTimeRange(c)
	if !ok {
		return
	}

	rows, err := MigrationManager.Queries.ListStatsApplication(c.Request.Context(), db.ListStatsApplicationParams{
		ContainerName: name,
		CapturedAt:    sql.NullTime{Time: from, Valid: true},
		CapturedAt_2:  sql.NullTime{Time: to, Valid: true},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, rows)
}
