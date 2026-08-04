package system

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// lockDir is where per-volume advisory lock files live. It's a package var
// (not a constant) so tests can point it at a temp directory — the real
// /etc/dokvol usually isn't writable by a non-root test run.
var lockDir = "/etc/dokvol/locks"

// VolumeLock is an exclusive, advisory lock on a single volume's source
// path, held for the duration of a migration or backup so the two
// subsystems (or two runs of the same one) can never operate on the same
// directory at once — nothing else prevents that today. It's a plain
// flock(2) on a marker file: if the holding process dies, the kernel drops
// the lock the moment the fd closes, so a crash never leaves it stuck.
type VolumeLock struct {
	f *os.File
}

// LockVolume acquires the lock for sourcePath without blocking: if another
// operation already holds it, this fails immediately rather than queuing —
// a silent queue would just look like a stuck job with no indication of why.
func LockVolume(sourcePath string) (*VolumeLock, error) {
	if err := os.MkdirAll(lockDir, 0700); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}

	sum := sha256.Sum256([]byte(sourcePath))
	lockPath := filepath.Join(lockDir, hex.EncodeToString(sum[:])+".lock")

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open lock file '%s': %w", lockPath, err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, NewAPIError(
			ErrMigrationVolumeLocked,
			fmt.Sprintf("volume '%s' is already being migrated or backed up", sourcePath),
			map[string]any{"source": sourcePath},
		)
	}

	return &VolumeLock{f: f}, nil
}

// Unlock releases the lock and closes the underlying file.
func (l *VolumeLock) Unlock() error {
	defer l.f.Close()
	return syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
}
