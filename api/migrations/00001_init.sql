-- +goose Up
CREATE TABLE IF NOT EXISTS drive (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    device      TEXT NOT NULL,
    mountpoint  TEXT NOT NULL,
    fstype      TEXT NOT NULL,
    total_gb    INTEGER NOT NULL,
    free_gb     INTEGER NOT NULL,
    used_pct    REAL NOT NULL,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS volume (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    container_name  TEXT NOT NULL,
    type            TEXT NOT NULL,
    source          TEXT NOT NULL,
    destination     TEXT NOT NULL,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS volume_drive (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    volume_id   INTEGER NOT NULL REFERENCES volume(id),
    drive_id    INTEGER NOT NULL REFERENCES drive(id),
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS volume_drive;
DROP TABLE IF EXISTS volume;
DROP TABLE IF EXISTS drive;
