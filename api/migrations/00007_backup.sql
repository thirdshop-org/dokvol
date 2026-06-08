-- +goose Up
CREATE TABLE backup_target (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    provider    TEXT NOT NULL CHECK(provider IN ('s3','sftp','local')),
    config      TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE backup_job (
    id          TEXT PRIMARY KEY,
    target_id   TEXT NOT NULL REFERENCES backup_target(id),
    app_name    TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    total_bytes INTEGER DEFAULT 0,
    duration_ms INTEGER DEFAULT 0,
    error_message TEXT,
    started_at  TIMESTAMP,
    completed_at TIMESTAMP,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE backup_job_volume (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id            TEXT NOT NULL REFERENCES backup_job(id),
    volume_name       TEXT NOT NULL,
    source_path       TEXT NOT NULL,
    backup_path       TEXT NOT NULL,
    total_bytes       INTEGER DEFAULT 0,
    transferred_bytes INTEGER DEFAULT 0,
    status            TEXT NOT NULL DEFAULT 'pending',
    error_message     TEXT,
    created_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE backup_schedule (
    id          TEXT PRIMARY KEY,
    target_id   TEXT NOT NULL REFERENCES backup_target(id),
    app_name    TEXT NOT NULL,
    cron_expr   TEXT NOT NULL,
    retention   INTEGER NOT NULL DEFAULT 7,
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_backup_job_target ON backup_job(target_id);
CREATE INDEX idx_backup_job_app ON backup_job(app_name);
CREATE INDEX idx_backup_job_status ON backup_job(status);
CREATE INDEX idx_backup_schedule_target ON backup_schedule(target_id);

-- +goose Down
DROP TABLE IF EXISTS backup_schedule;
DROP TABLE IF EXISTS backup_job_volume;
DROP TABLE IF EXISTS backup_job;
DROP TABLE IF EXISTS backup_target;
