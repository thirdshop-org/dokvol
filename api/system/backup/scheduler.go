package backup

import (
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"dokvol/api/system"
)

type BackupScheduler struct {
	DB     *sql.DB
	Engine *BackupEngine
	mu     sync.Mutex
	stopCh chan struct{}
	ticker *time.Ticker
}

func NewBackupScheduler(database *sql.DB, engine *BackupEngine) *BackupScheduler {
	return &BackupScheduler{
		DB:     database,
		Engine: engine,
		stopCh: make(chan struct{}),
	}
}

func (s *BackupScheduler) Start() {
	log.Println("backup scheduler: starting")
	s.checkAndRun()

	s.ticker = time.NewTicker(60 * time.Second)
	go func() {
		for {
			select {
			case <-s.ticker.C:
				s.checkAndRun()
			case <-s.stopCh:
				s.ticker.Stop()
				return
			}
		}
	}()
}

func (s *BackupScheduler) Stop() {
	close(s.stopCh)
}

func (s *BackupScheduler) checkAndRun() {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.DB.Query(
		`SELECT id, target_id, app_name, cron_expr, retention FROM backup_schedule WHERE enabled = 1`,
	)
	if err != nil {
		log.Printf("backup scheduler: query schedules: %s", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, targetID, appName, cronExpr string
		var retention int
		if err := rows.Scan(&id, &targetID, &appName, &cronExpr, &retention); err != nil {
			log.Printf("backup scheduler: scan schedule: %s", err)
			continue
		}

		if cronMatches(cronExpr, time.Now()) {
			log.Printf("backup scheduler: triggering backup for app=%s target=%s schedule=%s", appName, targetID, id)
			jobID, err := s.Engine.RunBackup(appName, targetID)
			if err != nil {
				log.Printf("backup scheduler: run backup failed: %s", err)
				continue
			}
			go s.applyRetention(targetID, appName, retention, jobID)
		}
	}
}

func cronMatches(expr string, t time.Time) bool {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return false
	}

	now := []int{t.Minute(), t.Hour(), t.Day(), int(t.Month()), int(t.Weekday())}
	names := []string{"minute", "hour", "day", "month", "weekday"}

	for i, field := range fields {
		if field == "*" {
			continue
		}
		if strings.Contains(field, "/") {
			parts := strings.SplitN(field, "/", 2)
			interval, err := strconv.Atoi(parts[1])
			if err != nil {
				return false
			}
			if interval == 0 {
				return false
			}
			if now[i]%interval != 0 {
				return false
			}
			continue
		}
		if strings.Contains(field, ",") {
			parts := strings.Split(field, ",")
			matched := false
			for _, p := range parts {
				if v, err := strconv.Atoi(p); err == nil && v == now[i] {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
			continue
		}
		if v, err := strconv.Atoi(field); err == nil && v == now[i] {
			continue
		}
		if field == "?" && (names[i] == "day" || names[i] == "weekday") {
			continue
		}
		return false
	}

	lastDay := daysInMonth(t.Year(), t.Month())
	if fields[2] == "L" {
		if t.Day() != lastDay {
			return false
		}
	}

	return true
}

func daysInMonth(year int, m time.Month) int {
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func (s *BackupScheduler) applyRetention(targetID, appName string, retention int, excludeJobID string) {
	if retention <= 0 {
		return
	}

	rows, err := s.DB.Query(
		`SELECT id, created_at FROM backup_job WHERE target_id = ? AND app_name = ? AND status = 'completed' ORDER BY created_at DESC`,
		targetID, appName,
	)
	if err != nil {
		log.Printf("backup scheduler: retention query: %s", err)
		return
	}
	defer rows.Close()

	type jobEntry struct {
		ID        string
		CreatedAt time.Time
	}

	var jobs []jobEntry
	for rows.Next() {
		var j jobEntry
		var createdAt time.Time
		if err := rows.Scan(&j.ID, &createdAt); err != nil {
			continue
		}
		j.CreatedAt = createdAt
		jobs = append(jobs, j)
	}

	if len(jobs) <= retention {
		return
	}

	target, err := GetTarget(s.DB, targetID)
	if err != nil {
		log.Printf("backup scheduler: get target for retention: %s", err)
		return
	}

	for _, j := range jobs[retention:] {
		if j.ID == excludeJobID {
			continue
		}

		volRows, err := s.DB.Query(
			`SELECT backup_path FROM backup_job_volume WHERE job_id = ?`, j.ID,
		)
		if err != nil {
			continue
		}

		var paths []string
		for volRows.Next() {
			var p string
			volRows.Scan(&p)
			paths = append(paths, p)
		}
		volRows.Close()

		if len(paths) == 0 {
			continue
		}

		configJSON, err := decryptTargetConfig(s.DB, targetID)
		if err != nil {
			log.Printf("backup scheduler: decrypt config for retention: %s", err)
			continue
		}

		rcloneConfig, err := BuildRcloneConfig(target.Provider, configJSON)
		if err != nil {
			continue
		}

		for _, p := range paths {
			cmd := exec.Command("rclone", "purge",
				"--config", "/dev/stdin",
				fmt.Sprintf("backup-target:%s", p),
			)
			cmd.Stdin = strings.NewReader(rcloneConfig)
			if err := cmd.Run(); err != nil {
				log.Printf("backup scheduler: purge old backup %s: %s", p, err)
			}
		}

		s.DB.Exec(`DELETE FROM backup_job_volume WHERE job_id = ?`, j.ID)
		s.DB.Exec(`DELETE FROM backup_job WHERE id = ?`, j.ID)

		log.Printf("backup scheduler: removed old backup job %s (retention=%d)", j.ID, retention)
	}
}

func decryptTargetConfig(db *sql.DB, targetID string) (string, error) {
	t, err := GetTarget(db, targetID)
	if err != nil {
		return "", err
	}
	return system.DecryptConfig(t.Config)
}
