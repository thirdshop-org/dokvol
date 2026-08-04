CREATE TABLE drive (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    device      TEXT NOT NULL,
    mountpoint  TEXT NOT NULL,
    fstype      TEXT NOT NULL,
    total_gb    INTEGER NOT NULL,
    free_gb     INTEGER NOT NULL,
    used_pct    REAL NOT NULL,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE volume (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    container_name  TEXT NOT NULL,
    type            TEXT NOT NULL,
    source          TEXT NOT NULL,
    destination     TEXT NOT NULL,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE volume_drive (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    volume_id   INTEGER NOT NULL REFERENCES volume(id),
    drive_id    INTEGER NOT NULL REFERENCES drive(id),
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE migration_job (
    id          TEXT PRIMARY KEY,
    app_name    TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE migration_volume_progress (
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
    backup_path       TEXT,
    created_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_preferences (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE stats_batch (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    captured_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE stats_volume (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    batch_id       INTEGER NOT NULL REFERENCES stats_batch(id),
    volume_name    TEXT NOT NULL,
    container_name TEXT NOT NULL,
    source_path    TEXT NOT NULL,
    total_bytes    INTEGER NOT NULL,
    duration_ms    INTEGER NOT NULL DEFAULT 0,
    captured_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE stats_drive (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    batch_id    INTEGER NOT NULL REFERENCES stats_batch(id),
    mountpoint  TEXT NOT NULL,
    device      TEXT NOT NULL,
    total_bytes INTEGER NOT NULL,
    used_bytes  INTEGER NOT NULL,
    free_bytes  INTEGER NOT NULL,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    captured_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE migration_log (
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

CREATE INDEX IF NOT EXISTS idx_stats_volume_name     ON stats_volume(volume_name, captured_at);
CREATE INDEX IF NOT EXISTS idx_stats_drive_mountpoint ON stats_drive(mountpoint, captured_at);

CREATE TABLE users (
    id                       INTEGER PRIMARY KEY AUTOINCREMENT,
    email                    TEXT,
    username                 TEXT NOT NULL UNIQUE,
    password_hash            TEXT NOT NULL,
    role                     TEXT NOT NULL DEFAULT 'user',
    password_change_required INTEGER NOT NULL DEFAULT 0,
    created_at               TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at               TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE refresh_tokens (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_refresh_tokens_token ON refresh_tokens(token);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
