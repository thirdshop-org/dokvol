-- +goose Up
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

-- +goose Down
DROP TABLE users;
