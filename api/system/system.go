package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/moby/moby/client"
)

const (
	DOKVOL_FOLDER        = ".dokvol"
	DOKVOL_METADATA_FILE = "metadata.json"
	VERSION              = "0.0.1"
)

type System struct {
	Drives       []DriveInfo
	Applications []Application
	docker       *client.Client // client Docker
}

func New() (*System, error) {

	docker, err := client.New(
		client.FromEnv,
	)
	if err != nil {
		return nil, NewAPIError(
			ErrSystemNotFound,
			fmt.Sprintf("failed to create docker client: %s", err),
			nil,
		)
	}

	drives := GetDrives()

	apps, err := GetApplicationsDetails(drives)
	if err != nil {
		return nil, NewAPIError(
			ErrSystemNotFound,
			fmt.Sprintf("failed to list applications: %s", err),
			nil,
		)
	}

	return &System{
		Drives:       drives,
		Applications: apps,
		docker:       docker,
	}, nil
}

type DokvolMetadata struct {
	Version   string `json:"version"`
	CreatedAt string `json:"created_at"`
}

func (s *System) driveExists(drive DriveInfo) bool {
	for _, systemDrive := range s.Drives {
		if systemDrive == drive {
			return true
		}
	}
	return false
}

func (s *System) CreateDokvolPartitionDriveFolder(drive DriveInfo) error {
	if err := s.createDokvolPartitionDriveFolder(drive); err != nil {
		return err
	}
	if err := s.createDokvolPartitionDriveMetadataFile(drive); err != nil {
		return err
	}
	if err := s.CheckDokvolHealth(drive); err != nil {
		return fmt.Errorf("dokvol created but health check failed: %w", err)
	}
	return nil
}

func (s *System) createDokvolPartitionDriveFolder(drive DriveInfo) error {
	if !s.driveExists(drive) {
		return NewAPIError(
			ErrDriveNotFound,
			"drive doesn't exist on the system",
			map[string]any{"mountpoint": drive.Mountpoint},
		)
	}

	dokvolPath := filepath.Join(drive.Mountpoint, DOKVOL_FOLDER)

	if _, err := os.Stat(dokvolPath); os.IsNotExist(err) {
		return os.Mkdir(dokvolPath, 0700)
	}

	return nil
}

func (s *System) createDokvolPartitionDriveMetadataFile(drive DriveInfo) error {
	if !s.driveExists(drive) {
		return NewAPIError(
			ErrDriveNotFound,
			"drive doesn't exist on the system",
			map[string]any{"mountpoint": drive.Mountpoint},
		)
	}

	dokvolPath := filepath.Join(drive.Mountpoint, DOKVOL_FOLDER)

	if _, err := os.Stat(dokvolPath); os.IsNotExist(err) {
		return NewAPIError(
			ErrDriveHealthCheck,
			fmt.Sprintf("dokvol folder doesn't exist at %s", dokvolPath),
			map[string]any{"path": dokvolPath},
		)
	}

	metadataPath := filepath.Join(dokvolPath, DOKVOL_METADATA_FILE)

	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		meta := DokvolMetadata{
			Version:   VERSION,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}

		bytes, err := json.Marshal(meta)
		if err != nil {
			return NewAPIError(
				ErrDriveHealthCheck,
				fmt.Sprintf("error marshaling metadata: %s", err),
				nil,
			)
		}

		return os.WriteFile(metadataPath, bytes, 0600)
	}

	return nil
}

func (s *System) CheckDokvolHealth(drive DriveInfo) error {
	if err := s.checkDokvolFolderHealth(drive); err != nil {
		return err
	}
	if err := s.checkDokvolMetadataHealth(drive); err != nil {
		return err
	}
	return nil
}

func (s *System) checkDokvolFolderHealth(drive DriveInfo) error {
	if !s.driveExists(drive) {
		return NewAPIError(
			ErrDriveNotFound,
			"drive doesn't exist on the system",
			map[string]any{"mountpoint": drive.Mountpoint},
		)
	}

	dokvolPath := filepath.Join(drive.Mountpoint, DOKVOL_FOLDER)

	info, err := os.Stat(dokvolPath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewAPIError(
				ErrDriveHealthCheck,
				fmt.Sprintf("dokvol folder missing at %s", dokvolPath),
				map[string]any{"path": dokvolPath},
			)
		}
		return NewAPIError(
			ErrDriveHealthCheck,
			fmt.Sprintf("cannot stat dokvol folder: %s", err),
			map[string]any{"path": dokvolPath},
		)
	}

	if !info.IsDir() {
		return NewAPIError(
			ErrDriveHealthCheck,
			fmt.Sprintf("dokvol path exists but is not a directory: %s", dokvolPath),
			map[string]any{"path": dokvolPath},
		)
	}

	testFile := filepath.Join(dokvolPath, ".write_test")
	if err := os.WriteFile(testFile, []byte{}, 0600); err != nil {
		return NewAPIError(
			ErrDriveHealthCheck,
			fmt.Sprintf("cannot write to dokvol folder: %s", err),
			map[string]any{"path": dokvolPath},
		)
	}
	os.Remove(testFile)

	return nil
}

func (s *System) checkDokvolMetadataHealth(drive DriveInfo) error {
	if !s.driveExists(drive) {
		return NewAPIError(
			ErrDriveNotFound,
			"drive doesn't exist on the system",
			map[string]any{"mountpoint": drive.Mountpoint},
		)
	}

	metadataPath := filepath.Join(drive.Mountpoint, DOKVOL_FOLDER, DOKVOL_METADATA_FILE)

	info, err := os.Stat(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewAPIError(
				ErrDriveHealthCheck,
				fmt.Sprintf("dokvol metadata file missing at %s", metadataPath),
				map[string]any{"path": metadataPath},
			)
		}
		return NewAPIError(
			ErrDriveHealthCheck,
			fmt.Sprintf("cannot stat dokvol metadata file: %s", err),
			map[string]any{"path": metadataPath},
		)
	}

	if info.IsDir() {
		return NewAPIError(
			ErrDriveHealthCheck,
			fmt.Sprintf("dokvol metadata path exists but is a directory: %s", metadataPath),
			map[string]any{"path": metadataPath},
		)
	}

	testFile := filepath.Join(filepath.Dir(metadataPath), ".write_test")
	if err := os.WriteFile(testFile, []byte{}, 0600); err != nil {
		return NewAPIError(
			ErrDriveHealthCheck,
			fmt.Sprintf("cannot write to dokvol folder: %s", err),
			map[string]any{"path": metadataPath},
		)
	}
	os.Remove(testFile)

	if info.Mode().Perm() != 0600 {
		return NewAPIError(
			ErrDriveHealthCheck,
			fmt.Sprintf("metadata file permissions mismatch: got %o, expected %o", info.Mode().Perm(), 0600),
			map[string]any{"path": metadataPath},
		)
	}

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return NewAPIError(
			ErrDriveHealthCheck,
			fmt.Sprintf("cannot read metadata file: %s", err),
			map[string]any{"path": metadataPath},
		)
	}

	var meta DokvolMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return NewAPIError(
			ErrDriveHealthCheck,
			fmt.Sprintf("metadata file is not valid JSON: %s", err),
			map[string]any{"path": metadataPath},
		)
	}

	if meta.Version == "" {
		return NewAPIError(
			ErrDriveHealthCheck,
			"metadata file is missing version field",
			map[string]any{"path": metadataPath},
		)
	}

	if meta.CreatedAt == "" {
		return NewAPIError(
			ErrDriveHealthCheck,
			"metadata file is missing created_at field",
			map[string]any{"path": metadataPath},
		)
	}

	return nil
}
