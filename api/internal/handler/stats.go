package handler

import (
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
		if t, err := time.Parse(time.RFC3339Nano, q.From); err == nil {
			from = t
		}
	}
	if q.To != "" {
		if t, err := time.Parse(time.RFC3339Nano, q.To); err == nil {
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
		CapturedAt:  from,
		CapturedAt_2: to,
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
		CapturedAt:  from,
		CapturedAt_2: to,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, rows)
}

type statsApplicationResponse struct {
	CapturedAt    time.Time `json:"captured_at"`
	ContainerName string    `json:"container_name"`
	TotalBytes    *float64  `json:"total_bytes"`
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
		CapturedAt:    from,
		CapturedAt_2:  to,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError("INTERNAL_ERROR", err.Error(), nil))
		return
	}

	resp := make([]statsApplicationResponse, len(rows))
	for i, r := range rows {
		var v *float64
		if r.TotalBytes.Valid {
			v = &r.TotalBytes.Float64
		}
		resp[i] = statsApplicationResponse{
			CapturedAt:    r.CapturedAt,
			ContainerName: r.ContainerName,
			TotalBytes:    v,
		}
	}

	c.JSON(http.StatusOK, resp)
}
