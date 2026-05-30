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

CREATE INDEX IF NOT EXISTS idx_stats_volume_name     ON stats_volume(volume_name, captured_at);
CREATE INDEX IF NOT EXISTS idx_stats_drive_mountpoint ON stats_drive(mountpoint, captured_at);
