-- +goose Up
CREATE TABLE IF NOT EXISTS migration_log (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id        TEXT    NOT NULL,
    app_name      TEXT    NOT NULL,
    volume_name   TEXT    NOT NULL,
    source_path   TEXT    NOT NULL,
    source_drive  TEXT,
    dest_path     TEXT    NOT NULL,
    dest_drive    TEXT    NOT NULL,
    total_bytes   INTEGER NOT NULL DEFAULT 0,
    duration_ms   INTEGER NOT NULL DEFAULT 0,
    status        TEXT    NOT NULL,
    error_message TEXT,
    started_at    TIMESTAMP,
    completed_at  TIMESTAMP,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_migration_log_app     ON migration_log(app_name);
CREATE INDEX IF NOT EXISTS idx_migration_log_drive   ON migration_log(dest_drive);
CREATE INDEX IF NOT EXISTS idx_migration_log_status  ON migration_log(status);
CREATE INDEX IF NOT EXISTS idx_migration_log_created ON migration_log(created_at);

-- +goose Down
DROP TABLE IF EXISTS migration_log;
