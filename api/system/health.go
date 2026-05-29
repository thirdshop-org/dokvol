package system

import (
	"fmt"
	"os/exec"
	"strings"
)

const (
	ErrSystemHealth = "SYSTEM.HEALTH_CHECK_FAILED"
)

func CheckSystemHealth() *APIError {
	var failures []string

	if _, err := exec.LookPath("rsync"); err != nil {
		failures = append(failures, "rsync not found in PATH")
	}

	if len(failures) == 0 {
		return nil
	}

	return NewAPIError(
		ErrSystemHealth,
		fmt.Sprintf("system health check failed: %s", strings.Join(failures, "; ")),
		map[string]any{"checks": failures},
	)
}
