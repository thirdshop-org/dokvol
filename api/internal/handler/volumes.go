package handler

import (
	"errors"
	"fmt"
	"net/http"

	"dokvol/api/system"

	"github.com/gin-gonic/gin"
)

func GetVolumes(c *gin.Context) {
	apps := system.GetDockerVolumesByContainers()
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

type migrateVolumeRequest struct {
	Application          string                     `json:"application"`
	DestinationMountpoint string                    `json:"destination_mountpoint"`
	Volumes              []migrateVolumeEntry       `json:"volumes"`
}

type migrateVolumeEntry struct {
	Name                 string `json:"name"`
	DestinationMountpoint string `json:"destination_mountpoint"`
}

type migrateVolumeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

func MigrateVolume(c *gin.Context) {
	var req migrateVolumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, migrateVolumeResponse{
			Success: false,
			Message: "invalid request body: " + err.Error(),
		})
		return
	}

	if req.Application == "" {
		c.JSON(http.StatusBadRequest, migrateVolumeResponse{
			Success: false,
			Message: "'application' is required",
		})
		return
	}

	if req.DestinationMountpoint == "" && len(req.Volumes) == 0 {
		c.JSON(http.StatusBadRequest, migrateVolumeResponse{
			Success: false,
			Message: "provide either 'destination_mountpoint' (all volumes) or 'volumes' (per-volume)",
		})
		return
	}

	if req.DestinationMountpoint != "" && len(req.Volumes) > 0 {
		c.JSON(http.StatusBadRequest, migrateVolumeResponse{
			Success: false,
			Message: "cannot set both 'destination_mountpoint' and 'volumes'",
		})
		return
	}

	s, err := system.New()
	if err != nil {
		c.JSON(http.StatusInternalServerError, migrateVolumeResponse{
			Success: false,
			Message: fmt.Sprintf("failed to initialize system: %s", err),
		})
		return
	}

	app := system.Application{Name: req.Application}
	opts := system.MoveStorageOptions{
		Application: app,
	}

	if req.DestinationMountpoint != "" {
		drive, err := findDriveByMountpoint(req.DestinationMountpoint)
		if err != nil {
			c.JSON(http.StatusNotFound, migrateVolumeResponse{
				Success: false,
				Message: err.Error(),
			})
			return
		}
		opts.DefaultDestinationDrive = drive
	} else {
		drives := system.GetDrives()
		volumeOpts := make([]system.ApplicationVolumeOptions, len(req.Volumes))
		for i, v := range req.Volumes {
			if v.Name == "" {
				c.JSON(http.StatusBadRequest, migrateVolumeResponse{
					Success: false,
					Message: fmt.Sprintf("'volumes[%d].name' is required", i),
				})
				return
			}
			if v.DestinationMountpoint == "" {
				c.JSON(http.StatusBadRequest, migrateVolumeResponse{
					Success: false,
					Message: fmt.Sprintf("'volumes[%d].destination_mountpoint' is required", i),
				})
				return
			}

			var volDetail *system.VolumeDetail
			for _, a := range s.Applications {
				if a.Name == req.Application {
					for _, vol := range a.DockerVolumes {
						if vol.Name == v.Name {
							volDetail = &vol
							break
						}
					}
				}
			}
			if volDetail == nil {
				c.JSON(http.StatusNotFound, migrateVolumeResponse{
					Success: false,
					Message: fmt.Sprintf("volume '%s' not found for application '%s'", v.Name, req.Application),
				})
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
				c.JSON(http.StatusNotFound, migrateVolumeResponse{
					Success: false,
					Message: fmt.Sprintf("no drive found with mountpoint '%s'", v.DestinationMountpoint),
				})
				return
			}

			volumeOpts[i] = system.ApplicationVolumeOptions{
				VolumeDetail:     *volDetail,
				DestinationDrive: *destDrive,
			}
		}
		opts.ApplicationVolumes = &volumeOpts
	}

	if err := s.MoveApplicationStorage(opts); err != nil {
		var apiErr *system.APIError
		if errors.As(err, &apiErr) {
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

	c.JSON(http.StatusOK, migrateVolumeResponse{
		Success: true,
		Message: fmt.Sprintf("successfully migrated application '%s'", req.Application),
	})
}
