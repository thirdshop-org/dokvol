package backup

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"dokvol/api/system"

	"github.com/google/uuid"
)

type ProviderType string

const (
	ProviderS3    ProviderType = "s3"
	ProviderSFTP  ProviderType = "sftp"
	ProviderLocal ProviderType = "local"
)

type S3Config struct {
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	PathStyle bool   `json:"path_style"`
	Prefix    string `json:"prefix,omitempty"`
}

type SFTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password,omitempty"`
	KeyPath  string `json:"key_path,omitempty"`
	BasePath string `json:"base_path"`
}

type LocalConfig struct {
	Path string `json:"path"`
}

type BackupTarget struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Provider  ProviderType `json:"provider"`
	Config    string       `json:"-"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

type BackupJob struct {
	ID           string    `json:"id"`
	TargetID     string    `json:"target_id"`
	AppName      string    `json:"app_name"`
	Status       string    `json:"status"`
	TotalBytes   int64     `json:"total_bytes"`
	DurationMs   int64     `json:"duration_ms"`
	ErrorMessage string    `json:"error_message,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type BackupSchedule struct {
	ID        string    `json:"id"`
	TargetID  string    `json:"target_id"`
	AppName   string    `json:"app_name"`
	CronExpr  string    `json:"cron_expr"`
	Retention int       `json:"retention"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func CreateTarget(db *sql.DB, name string, provider ProviderType, config interface{}) (*BackupTarget, error) {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	encrypted, err := system.EncryptConfig(string(configBytes))
	if err != nil {
		return nil, fmt.Errorf("encrypt config: %w", err)
	}

	id := uuid.New().String()
	now := time.Now()

	_, err = db.Exec(
		`INSERT INTO backup_target (id, name, provider, config, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, name, string(provider), encrypted, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert target: %w", err)
	}

	return &BackupTarget{
		ID:        id,
		Name:      name,
		Provider:  provider,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func GetTarget(db *sql.DB, id string) (*BackupTarget, error) {
	var t BackupTarget
	var createdAt, updatedAt time.Time
	err := db.QueryRow(
		`SELECT id, name, provider, config, created_at, updated_at FROM backup_target WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Provider, &t.Config, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("get target: %w", err)
	}
	t.CreatedAt = createdAt
	t.UpdatedAt = updatedAt
	return &t, nil
}

func ListTargets(dbRaw *sql.DB) ([]BackupTarget, error) {
	rows, err := dbRaw.Query(`SELECT id, name, provider, config, created_at, updated_at FROM backup_target ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list targets: %w", err)
	}
	defer rows.Close()

	var targets []BackupTarget
	for rows.Next() {
		var t BackupTarget
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&t.ID, &t.Name, &t.Provider, &t.Config, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan target: %w", err)
		}
		t.CreatedAt = createdAt
		t.UpdatedAt = updatedAt
		targets = append(targets, t)
	}
	return targets, nil
}

func UpdateTarget(db *sql.DB, id, name string, provider ProviderType, config interface{}) error {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	encrypted, err := system.EncryptConfig(string(configBytes))
	if err != nil {
		return fmt.Errorf("encrypt config: %w", err)
	}
	_, err = db.Exec(
		`UPDATE backup_target SET name = ?, provider = ?, config = ?, updated_at = ? WHERE id = ?`,
		name, string(provider), encrypted, time.Now(), id,
	)
	return err
}

func DeleteTarget(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM backup_target WHERE id = ?`, id)
	return err
}

func GetTargetDecryptedConfig(db *sql.DB, id string) (string, error) {
	t, err := GetTarget(db, id)
	if err != nil {
		return "", err
	}
	decrypted, err := system.DecryptConfig(t.Config)
	if err != nil {
		return "", fmt.Errorf("decrypt config: %w", err)
	}
	return decrypted, nil
}

func CreateSchedule(db *sql.DB, targetID, appName, cronExpr string, retention int) (*BackupSchedule, error) {
	id := uuid.New().String()
	now := time.Now()
	_, err := db.Exec(
		`INSERT INTO backup_schedule (id, target_id, app_name, cron_expr, retention, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, targetID, appName, cronExpr, retention, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create schedule: %w", err)
	}
	return &BackupSchedule{ID: id, TargetID: targetID, AppName: appName, CronExpr: cronExpr, Retention: retention, Enabled: true, CreatedAt: now, UpdatedAt: now}, nil
}

func UpdateSchedule(db *sql.DB, id, appName, cronExpr string, retention int, enabled *bool) error {
	if enabled != nil {
		val := 0
		if *enabled { val = 1 }
		_, err := db.Exec(
			`UPDATE backup_schedule SET app_name = ?, cron_expr = ?, retention = ?, enabled = ?, updated_at = ? WHERE id = ?`,
			appName, cronExpr, retention, val, time.Now(), id,
		)
		return err
	}
	_, err := db.Exec(
		`UPDATE backup_schedule SET app_name = ?, cron_expr = ?, retention = ?, updated_at = ? WHERE id = ?`,
		appName, cronExpr, retention, time.Now(), id,
	)
	return err
}

func DeleteSchedule(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM backup_schedule WHERE id = ?`, id)
	return err
}
