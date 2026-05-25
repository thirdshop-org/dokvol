package handler

import (
	"fmt"
	"net/http"

	"dokvol/api/system"

	"github.com/gin-gonic/gin"
)

type driveActionRequest struct {
	Mountpoint string `json:"mountpoint" form:"mountpoint"`
}

type initDriveResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type healthCheckResponse struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

func findDriveByMountpoint(mountpoint string) (*system.DriveInfo, error) {
	drives := system.GetDrives()
	for _, d := range drives {
		if d.Mountpoint == mountpoint {
			return &d, nil
		}
	}
	return nil, fmt.Errorf("no drive found with mountpoint %q", mountpoint)
}

func GetDrives(c *gin.Context) {
	drives := system.GetDrives()
	if drives == nil {
		drives = []system.DriveInfo{}
	}
	c.JSON(http.StatusOK, drives)
}

func InitDrive(c *gin.Context) {
	var req driveActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, initDriveResponse{
			Success: false,
			Message: "invalid request body, 'mountpoint' required",
		})
		return
	}
	if req.Mountpoint == "" {
		c.JSON(http.StatusBadRequest, initDriveResponse{
			Success: false,
			Message: "'mountpoint' is required",
		})
		return
	}

	drive, err := findDriveByMountpoint(req.Mountpoint)
	if err != nil {
		c.JSON(http.StatusNotFound, initDriveResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	s := system.System{Drives: system.GetDrives()}
	if err := s.CreateDokvolPartitionDriveFolder(*drive); err != nil {
		c.JSON(http.StatusInternalServerError, initDriveResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, initDriveResponse{
		Success: true,
		Message: fmt.Sprintf("DokVol initialized on %s (%s)", drive.Mountpoint, drive.Device),
	})
}

func CheckDriveHealth(c *gin.Context) {
	var req driveActionRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, healthCheckResponse{
			Healthy: false,
			Message: "invalid query parameters",
		})
		return
	}
	if req.Mountpoint == "" {
		c.JSON(http.StatusBadRequest, healthCheckResponse{
			Healthy: false,
			Message: "'mountpoint' query parameter is required",
		})
		return
	}

	drive, err := findDriveByMountpoint(req.Mountpoint)
	if err != nil {
		c.JSON(http.StatusNotFound, healthCheckResponse{
			Healthy: false,
			Message: err.Error(),
		})
		return
	}

	s := system.System{Drives: system.GetDrives()}
	if err := s.CheckDokvolHealth(*drive); err != nil {
		c.JSON(http.StatusOK, healthCheckResponse{
			Healthy: false,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, healthCheckResponse{
		Healthy: true,
		Message: fmt.Sprintf("DokVol storage is healthy on %s (%s)", drive.Mountpoint, drive.Device),
	})
}
