-- +goose Up
ALTER TABLE migration_volume_progress ADD COLUMN backup_path TEXT;

-- +goose Down
ALTER TABLE migration_volume_progress DROP COLUMN backup_path;
