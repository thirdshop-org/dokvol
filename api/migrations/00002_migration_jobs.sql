-- +goose Up
CREATE TABLE IF NOT EXISTS migration_job (
    id          TEXT PRIMARY KEY,
    app_name    TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS migration_volume_progress (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id            TEXT NOT NULL REFERENCES migration_job(id),
    volume_name       TEXT NOT NULL,
    source_path       TEXT NOT NULL,
    dest_path         TEXT NOT NULL,
    dest_drive        TEXT NOT NULL,
    step              TEXT NOT NULL DEFAULT 'pending',
    total_bytes       INTEGER NOT NULL DEFAULT 0,
    transferred_bytes INTEGER NOT NULL DEFAULT 0,
    error_message     TEXT,
    created_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS migration_volume_progress;
DROP TABLE IF EXISTS migration_job;
