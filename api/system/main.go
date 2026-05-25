package system

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

const (
	DOKVOL_FOLDER        = ".dokvol"
	DOKVOL_METADATA_FILE = "metadata.json"
	VERSION              = "0.0.1"
)

type System struct {
	Drives       []DriveInfo
	Applications []Application
}

func New() System {

	drives := GetDrives()

	return System{
		Drives:       drives,
		Applications: GetApplicationsDetails(drives),
	}
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
		return fmt.Errorf("drive doesn't exist on the system")
	}

	dokvolPath := filepath.Join(drive.Mountpoint, DOKVOL_FOLDER)

	if _, err := os.Stat(dokvolPath); os.IsNotExist(err) {
		return os.Mkdir(dokvolPath, 0700)
	}

	return nil
}

func (s *System) createDokvolPartitionDriveMetadataFile(drive DriveInfo) error {
	if !s.driveExists(drive) {
		return fmt.Errorf("drive doesn't exist on the system")
	}

	dokvolPath := filepath.Join(drive.Mountpoint, DOKVOL_FOLDER)

	if _, err := os.Stat(dokvolPath); os.IsNotExist(err) {
		return fmt.Errorf("dokvol folder doesn't exist at %s", dokvolPath)
	}

	metadataPath := filepath.Join(dokvolPath, DOKVOL_METADATA_FILE)

	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		meta := DokvolMetadata{
			Version:   VERSION,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}

		bytes, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("error marshaling metadata: %w", err)
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
		return fmt.Errorf("drive doesn't exist on the system")
	}

	dokvolPath := filepath.Join(drive.Mountpoint, DOKVOL_FOLDER)

	info, err := os.Stat(dokvolPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("dokvol folder missing at %s", dokvolPath)
		}
		return fmt.Errorf("cannot stat dokvol folder: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("dokvol path exists but is not a directory: %s", dokvolPath)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot read syscall stats for dokvol folder")
	}

	currentUser, err := user.Lookup("dokvol")
	if err != nil {
		return fmt.Errorf("cannot find dokvol system user: %w", err)
	}

	expectedUID, _ := strconv.Atoi(currentUser.Uid)
	expectedGID, _ := strconv.Atoi(currentUser.Gid)

	if int(stat.Uid) != expectedUID {
		return fmt.Errorf("dokvol folder owner mismatch: got uid=%d, expected uid=%d", stat.Uid, expectedUID)
	}

	if int(stat.Gid) != expectedGID {
		return fmt.Errorf("dokvol folder group mismatch: got gid=%d, expected gid=%d", stat.Gid, expectedGID)
	}

	if info.Mode().Perm() != 0700 {
		return fmt.Errorf("dokvol folder permissions mismatch: got %o, expected %o", info.Mode().Perm(), 0700)
	}

	return nil
}

func (s *System) checkDokvolMetadataHealth(drive DriveInfo) error {
	if !s.driveExists(drive) {
		return fmt.Errorf("drive doesn't exist on the system")
	}

	metadataPath := filepath.Join(drive.Mountpoint, DOKVOL_FOLDER, DOKVOL_METADATA_FILE)

	info, err := os.Stat(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("dokvol metadata file missing at %s", metadataPath)
		}
		return fmt.Errorf("cannot stat dokvol metadata file: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("dokvol metadata path exists but is a directory: %s", metadataPath)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot read syscall stats for dokvol metadata file")
	}

	currentUser, err := user.Lookup("dokvol")
	if err != nil {
		return fmt.Errorf("cannot find dokvol system user: %w", err)
	}

	expectedUID, _ := strconv.Atoi(currentUser.Uid)
	expectedGID, _ := strconv.Atoi(currentUser.Gid)

	if int(stat.Uid) != expectedUID {
		return fmt.Errorf("metadata file owner mismatch: got uid=%d, expected uid=%d", stat.Uid, expectedUID)
	}

	if int(stat.Gid) != expectedGID {
		return fmt.Errorf("metadata file group mismatch: got gid=%d, expected gid=%d", stat.Gid, expectedGID)
	}

	if info.Mode().Perm() != 0600 {
		return fmt.Errorf("metadata file permissions mismatch: got %o, expected %o", info.Mode().Perm(), 0600)
	}

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("cannot read metadata file: %w", err)
	}

	var meta DokvolMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("metadata file is not valid JSON: %w", err)
	}

	if meta.Version == "" {
		return fmt.Errorf("metadata file is missing version field")
	}

	if meta.CreatedAt == "" {
		return fmt.Errorf("metadata file is missing created_at field")
	}

	return nil
}
