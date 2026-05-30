-- +goose Up
CREATE TABLE IF NOT EXISTS user_preferences (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO user_preferences (key, value) VALUES
    ('stats_interval_seconds', '600'),
    ('stats_retention_days', '30');

CREATE TABLE IF NOT EXISTS stats_batch (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    captured_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS stats_volume (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    batch_id       INTEGER NOT NULL REFERENCES stats_batch(id),
    volume_name    TEXT NOT NULL,
    container_name TEXT NOT NULL,
    source_path    TEXT NOT NULL,
    total_bytes    INTEGER NOT NULL,
    duration_ms    INTEGER NOT NULL DEFAULT 0,
    captured_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS stats_drive (
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
CREATE INDEX IF NOT EXISTS idx_stats_batch_captured   ON stats_batch(captured_at);

-- +goose Down
DROP INDEX IF EXISTS idx_stats_batch_captured;
DROP INDEX IF EXISTS idx_stats_drive_mountpoint;
DROP INDEX IF EXISTS idx_stats_volume_name;
DROP TABLE IF EXISTS stats_drive;
DROP TABLE IF EXISTS stats_volume;
DROP TABLE IF EXISTS stats_batch;
DROP TABLE IF EXISTS user_preferences;
