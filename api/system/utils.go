package system

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

func dirSize(path string) (int64, error) {

	var size int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

func availableDiskSpace(path string) (int64, error) {

	var stat syscall.Statfs_t

	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}

	// Blocks disponibles × taille d'un block
	return int64(stat.Bavail) * stat.Bsize, nil
}
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func compareFileChecksum(sourcePath, destPath string) error {

	sourceHash, err := hashFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to hash source: %w", err)
	}

	destHash, err := hashFile(destPath)
	if err != nil {
		return fmt.Errorf("failed to hash destination: %w", err)
	}

	if sourceHash != destHash {
		return fmt.Errorf("source(%s) != dest(%s)", sourceHash, destHash)
	}

	return nil
}

func hashFile(path string) (string, error) {

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()

	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// Option : vérification rapide par taille + sample
func quickVerify(sourcePath, destPath string) error {

	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	destInfo, err := os.Stat(destPath)
	if err != nil {
		return err
	}

	// Vérifier la taille
	if sourceInfo.Size() != destInfo.Size() {
		return fmt.Errorf("size mismatch: %d != %d", sourceInfo.Size(), destInfo.Size())
	}

	return nil
}
