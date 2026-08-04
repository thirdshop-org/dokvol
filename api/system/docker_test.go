package system

import (
	"path/filepath"
	"testing"
)

func TestIsSourceAccessible(t *testing.T) {
	dir := t.TempDir()

	if isSourceAccessible("") {
		t.Error("empty source should not be accessible")
	}
	if !isSourceAccessible(dir) {
		t.Errorf("existing directory %q should be accessible", dir)
	}
	if isSourceAccessible(filepath.Join(dir, "does-not-exist")) {
		t.Error("nonexistent path should not be accessible")
	}
}
