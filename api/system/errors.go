package system

import (
	"fmt"
	"net/http"
	"strings"
)

type APIError struct {
	Code    string         `json:"error_code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewAPIError(code, message string, details map[string]any) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

func (e *APIError) HTTPStatus() int {
	switch {
	case e.Code == "INTERNAL_ERROR":
		return http.StatusInternalServerError
	case strings.HasPrefix(e.Code, "SYSTEM."):
		return http.StatusInternalServerError
	case strings.HasPrefix(e.Code, "DRIVE.NOT_FOUND"):
		return http.StatusNotFound
	case strings.HasPrefix(e.Code, "DRIVE."):
		return http.StatusInternalServerError
	case e.Code == ErrAppNotFound:
		return http.StatusNotFound
	case e.Code == ErrAppNoVolumes:
		return http.StatusBadRequest
	case e.Code == ErrMigrationSameDrive:
		return http.StatusConflict
	case e.Code == ErrMigrationDiskSpace:
		return http.StatusConflict
	case strings.HasPrefix(e.Code, "MIGRATION.VOLUME"):
		return http.StatusNotFound
	case strings.HasPrefix(e.Code, "MIGRATION."):
		return http.StatusBadRequest
	case strings.HasPrefix(e.Code, "CONTAINER."):
		return http.StatusInternalServerError
	case strings.HasPrefix(e.Code, "STORAGE."):
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

const (
	ErrSystemNotFound        = "SYSTEM.NOT_FOUND"
	ErrDriveNotFound         = "DRIVE.NOT_FOUND"
	ErrDriveHealthCheck      = "DRIVE.HEALTH_CHECK_FAILED"
	ErrAppNotFound           = "APP.NOT_FOUND"
	ErrAppNoVolumes          = "APP.NO_VOLUMES"
	ErrMigrationAmbiguous    = "MIGRATION.AMBIGUOUS_OPTIONS"
	ErrMigrationSameDrive    = "MIGRATION.SAME_DRIVE"
	ErrMigrationNoDest       = "MIGRATION.NO_DESTINATION"
	ErrMigrationVolNotFound  = "MIGRATION.VOLUME_NOT_FOUND"
	ErrMigrationVolMismatch  = "MIGRATION.VOLUME_MISMATCH"
	ErrMigrationDiskSpace    = "MIGRATION.INSUFFICIENT_DISK_SPACE"
	ErrMigrationSyncFailed   = "MIGRATION.SYNC_FAILED"
	ErrMigrationVerifyFailed = "MIGRATION.VERIFY_FAILED"
	ErrMigrationRelinkFailed = "MIGRATION.RELINK_FAILED"
	ErrContainerStopFailed   = "CONTAINER.STOP_FAILED"
	ErrContainerStartFailed  = "CONTAINER.START_FAILED"
	ErrContainerTimeout      = "CONTAINER.TIMEOUT"
	ErrStorageChecksum       = "STORAGE.CHECKSUM_MISMATCH"
)
