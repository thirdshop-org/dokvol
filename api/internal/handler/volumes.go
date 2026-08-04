package handler

import (
	"fmt"
	"net/http"
	"time"

	"dokvol/api/system"

	"github.com/gin-gonic/gin"
)

type migrateRequest struct {
	Application          string               `json:"application"`
	DestinationMountpoint string              `json:"destination_mountpoint"`
	Volumes              []migrateVolEntry    `json:"volumes"`
}

type migrateVolEntry struct {
	Name                 string `json:"name"`
	DestinationMountpoint string `json:"destination_mountpoint"`
}

type startMigrationResponse struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

type volumeProgressJSON struct {
	VolumeName       string    `json:"volume_name"`
	Step             string    `json:"step"`
	TotalBytes       int64     `json:"total_bytes"`
	TransferredBytes int64     `json:"transferred_bytes"`
	Error            string    `json:"error,omitempty"`
	// BackupPath, when set, is where the pre-migration data was moved aside
	// to and has not yet been reclaimed — e.g. the job was interrupted, or
	// the container failed to restart on the new location.
	BackupPath string    `json:"backup_path,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type jobJSON struct {
	ID          string              `json:"id"`
	AppName     string              `json:"app_name"`
	Status      string              `json:"status"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	CompletedAt time.Time           `json:"completed_at"`
	Volumes     []volumeProgressJSON `json:"volumes"`
}

func jobToJSON(job *system.Job) jobJSON {
	vols := make([]volumeProgressJSON, len(job.Volumes))
	for i := range job.Volumes {
		vols[i] = volumeProgressJSON{
			VolumeName:       job.Volumes[i].VolumeName,
			Step:             job.Volumes[i].Step,
			TotalBytes:       job.Volumes[i].TotalBytes,
			TransferredBytes: job.Volumes[i].Transferred,
			Error:            job.Volumes[i].Error,
			BackupPath:       job.Volumes[i].BackupPath,
			CreatedAt:        job.Volumes[i].CreatedAt,
			UpdatedAt:        job.Volumes[i].UpdatedAt,
		}
	}
	return jobJSON{
		ID:          job.ID,
		AppName:     job.AppName,
		Status:      string(job.Status),
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
		CompletedAt: job.CompletedAt,
		Volumes:     vols,
	}
}

func GetVolumes(c *gin.Context) {
	apps, err := system.GetDockerVolumesByContainers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError(
			"INTERNAL_ERROR",
			fmt.Sprintf("failed to list volumes: %s", err),
			nil,
		))
		return
	}
	if apps == nil {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	var volumes []system.VolumeDetail
	for _, app := range apps {
		volumes = append(volumes, app.Volumes...)
	}

	if volumes == nil {
		volumes = []system.VolumeDetail{}
	}

	c.JSON(http.StatusOK, volumes)
}

func MigrateVolume(c *gin.Context) {
	var req migrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, system.NewAPIError(
			"INTERNAL_ERROR",
			"invalid request body: "+err.Error(),
			nil,
		))
		return
	}

	if req.Application == "" {
		c.JSON(http.StatusBadRequest, system.NewAPIError(
			"INTERNAL_ERROR",
			"'application' is required",
			nil,
		))
		return
	}

	if req.DestinationMountpoint == "" && len(req.Volumes) == 0 {
		c.JSON(http.StatusBadRequest, system.NewAPIError(
			"INTERNAL_ERROR",
			"provide either 'destination_mountpoint' (all volumes) or 'volumes' (per-volume)",
			nil,
		))
		return
	}

	if req.DestinationMountpoint != "" && len(req.Volumes) > 0 {
		c.JSON(http.StatusBadRequest, system.NewAPIError(
			"INTERNAL_ERROR",
			"cannot set both 'destination_mountpoint' and 'volumes'",
			nil,
		))
		return
	}

	s, err := system.New()
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError(
			"INTERNAL_ERROR",
			fmt.Sprintf("failed to initialize system: %s", err),
			nil,
		))
		return
	}

	var app *system.Application
	for i := range s.Applications {
		if s.Applications[i].Name == req.Application {
			app = &s.Applications[i]
			break
		}
	}
	if app == nil {
		c.JSON(http.StatusNotFound, system.NewAPIError(
			system.ErrAppNotFound,
			fmt.Sprintf("application '%s' not found", req.Application),
			nil,
		))
		return
	}

	var volumeOpts []system.ApplicationVolumeOptions

	if req.DestinationMountpoint != "" {
		destDrive, err := findDriveByMountpoint(req.DestinationMountpoint)
		if err != nil {
			c.JSON(http.StatusNotFound, system.NewAPIError(
				system.ErrDriveNotFound,
				err.Error(),
				nil,
			))
			return
		}
		for _, vol := range app.DockerVolumes {
			volumeOpts = append(volumeOpts, system.ApplicationVolumeOptions{
				VolumeDetail:     vol,
				DestinationDrive: *destDrive,
			})
		}
	} else {
		drives := system.GetDrives()
		for _, v := range req.Volumes {
			if v.Name == "" {
				c.JSON(http.StatusBadRequest, system.NewAPIError(
					"INTERNAL_ERROR",
					"'volumes[].name' is required",
					nil,
				))
				return
			}
			if v.DestinationMountpoint == "" {
				c.JSON(http.StatusBadRequest, system.NewAPIError(
					"INTERNAL_ERROR",
					"'volumes[].destination_mountpoint' is required",
					nil,
				))
				return
			}

			var volDetail *system.VolumeDetail
			for j := range app.DockerVolumes {
				if app.DockerVolumes[j].Name == v.Name {
					volDetail = &app.DockerVolumes[j]
					break
				}
			}
			if volDetail == nil {
				c.JSON(http.StatusNotFound, system.NewAPIError(
					system.ErrMigrationVolNotFound,
					fmt.Sprintf("volume '%s' not found for application '%s'", v.Name, req.Application),
					nil,
				))
				return
			}

			var destDrive *system.DriveInfo
			for _, d := range drives {
				if d.Mountpoint == v.DestinationMountpoint {
					destDrive = &d
					break
				}
			}
			if destDrive == nil {
				c.JSON(http.StatusNotFound, system.NewAPIError(
					system.ErrDriveNotFound,
					fmt.Sprintf("no drive found with mountpoint '%s'", v.DestinationMountpoint),
					nil,
				))
				return
			}

			volumeOpts = append(volumeOpts, system.ApplicationVolumeOptions{
				VolumeDetail:     *volDetail,
				DestinationDrive: *destDrive,
			})
		}
	}

	jobID, err := MigrationManager.StartJob(c.Request.Context(), req.Application, *app, volumeOpts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError(
			"INTERNAL_ERROR",
			fmt.Sprintf("failed to start migration: %s", err),
			nil,
		))
		return
	}

	c.JSON(http.StatusCreated, startMigrationResponse{
		JobID:  jobID,
		Status: "pending",
	})
}

func GetMigrationJobs(c *gin.Context) {
	jobs, err := MigrationManager.ListJobs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError(
			"INTERNAL_ERROR",
			err.Error(),
			nil,
		))
		return
	}

	result := make([]jobJSON, len(jobs))
	for i, j := range jobs {
		result[i] = jobToJSON(j)
	}
	if result == nil {
		result = []jobJSON{}
	}

	c.JSON(http.StatusOK, result)
}

type deleteVolumeEntry struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Type   string `json:"type"`
}

type deleteVolumeRequest struct {
	Volumes []deleteVolumeEntry `json:"volumes"`
}

type deleteVolumeResponse struct {
	Success bool     `json:"success"`
	Errors  []string `json:"errors,omitempty"`
}

func DeleteVolumes(c *gin.Context) {
	var req deleteVolumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, system.NewAPIError(
			"INTERNAL_ERROR",
			"invalid request body",
			nil,
		))
		return
	}

	if len(req.Volumes) == 0 {
		c.JSON(http.StatusBadRequest, system.NewAPIError(
			"INTERNAL_ERROR",
			"'volumes' is required",
			nil,
		))
		return
	}

	var vols []system.VolumeDetail
	for _, v := range req.Volumes {
		vols = append(vols, system.VolumeDetail{
			Name: v.Name,
			Source: v.Source,
			Type:   v.Type,
		})
	}

	errs := system.DeleteVolumes(vols)
	resp := deleteVolumeResponse{Success: len(errs) == 0}
	for _, e := range errs {
		resp.Errors = append(resp.Errors, e.Error())
	}
	c.JSON(http.StatusOK, resp)
}

func GetMigrationJob(c *gin.Context) {
	id := c.Param("id")
	job, err := MigrationManager.GetJob(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, system.NewAPIError(
			"INTERNAL_ERROR",
			fmt.Sprintf("job '%s' not found", id),
			nil,
		))
		return
	}

	c.JSON(http.StatusOK, jobToJSON(job))
}
