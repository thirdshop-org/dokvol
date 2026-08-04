package system

import (
	"testing"
)

func TestLockVolume_RejectsConcurrentLock(t *testing.T) {
	lockDir = t.TempDir()

	lock, err := LockVolume("/var/lib/docker/volumes/plex-config/_data")
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}

	if _, err := LockVolume("/var/lib/docker/volumes/plex-config/_data"); err == nil {
		t.Fatal("second lock on the same path should have failed while the first is held")
	}

	if err := lock.Unlock(); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	again, err := LockVolume("/var/lib/docker/volumes/plex-config/_data")
	if err != nil {
		t.Fatalf("lock after unlock: %v", err)
	}
	_ = again.Unlock()
}

func TestLockVolume_DifferentPathsDontContend(t *testing.T) {
	lockDir = t.TempDir()

	a, err := LockVolume("/var/lib/docker/volumes/a/_data")
	if err != nil {
		t.Fatalf("lock a: %v", err)
	}
	defer a.Unlock()

	b, err := LockVolume("/var/lib/docker/volumes/b/_data")
	if err != nil {
		t.Fatalf("lock b should not contend with a: %v", err)
	}
	defer b.Unlock()
}
