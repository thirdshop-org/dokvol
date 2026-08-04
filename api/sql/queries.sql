-- name: CreateDrive :one
INSERT INTO drive (device, mountpoint, fstype, total_gb, free_gb, used_pct)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetDrive :one
SELECT * FROM drive WHERE id = ?;

-- name: ListDrives :many
SELECT * FROM drive ORDER BY mountpoint;

-- name: DeleteDrive :exec
DELETE FROM drive WHERE id = ?;

-- name: CreateVolume :one
INSERT INTO volume (container_name, type, source, destination)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetVolume :one
SELECT * FROM volume WHERE id = ?;

-- name: ListVolumes :many
SELECT * FROM volume ORDER BY container_name;

-- name: DeleteVolume :exec
DELETE FROM volume WHERE id = ?;

-- name: CreateVolumeDrive :one
INSERT INTO volume_drive (volume_id, drive_id)
VALUES (?, ?)
RETURNING *;

-- name: GetVolumeDrive :one
SELECT * FROM volume_drive WHERE id = ?;

-- name: ListVolumeDrives :many
SELECT
    vd.id,
    vd.volume_id,
    vd.drive_id,
    vd.created_at,
    v.container_name,
    v.type AS volume_type,
    v.source,
    v.destination,
    d.device,
    d.mountpoint,
    d.fstype,
    d.total_gb,
    d.free_gb,
    d.used_pct
FROM volume_drive vd
JOIN volume v ON v.id = vd.volume_id
JOIN drive d ON d.id = vd.drive_id;

-- name: DeleteVolumeDrive :exec
DELETE FROM volume_drive WHERE id = ?;

-- name: CreateMigrationJob :one
INSERT INTO migration_job (id, app_name, status)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetMigrationJob :one
SELECT * FROM migration_job WHERE id = ?;

-- name: ListMigrationJobs :many
SELECT * FROM migration_job ORDER BY created_at DESC;

-- name: UpdateMigrationJobStatus :exec
UPDATE migration_job SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: CreateVolumeProgress :one
INSERT INTO migration_volume_progress (job_id, volume_name, source_path, dest_path, dest_drive, step)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListVolumeProgressByJob :many
SELECT * FROM migration_volume_progress WHERE job_id = ? ORDER BY id;

-- name: UpdateVolumeProgressStep :exec
UPDATE migration_volume_progress SET step = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: UpdateVolumeProgressBytes :exec
UPDATE migration_volume_progress SET transferred_bytes = ?, total_bytes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: UpdateVolumeProgressError :exec
UPDATE migration_volume_progress SET step = 'failed', error_message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: UpdateVolumeProgressBackupPath :exec
UPDATE migration_volume_progress SET backup_path = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: MarkVolumeProgressInterrupted :exec
UPDATE migration_volume_progress SET step = 'interrupted', error_message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: ListRunningMigrationJobs :many
SELECT * FROM migration_job WHERE status = 'running';

-- name: GetVolumeProgress :one
SELECT * FROM migration_volume_progress WHERE id = ?;

-- name: ListVolumeProgressWithBackupPath :many
SELECT
    p.id,
    p.job_id,
    p.volume_name,
    p.source_path,
    p.dest_path,
    p.dest_drive,
    p.step,
    p.backup_path,
    p.updated_at,
    j.app_name
FROM migration_volume_progress p
JOIN migration_job j ON j.id = p.job_id
WHERE p.backup_path IS NOT NULL AND p.backup_path != ''
ORDER BY p.updated_at DESC;

-- name: MarkVolumeProgressRestored :exec
UPDATE migration_volume_progress SET step = 'restored', backup_path = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: ListJobsWithProgress :many
SELECT
    j.id AS job_id,
    j.app_name,
    j.status,
    j.created_at AS job_created_at,
    j.updated_at AS job_updated_at,
    p.id AS progress_id,
    p.job_id AS p_job_id,
    p.volume_name,
    p.source_path,
    p.dest_path,
    p.dest_drive,
    p.step,
    p.total_bytes,
    p.transferred_bytes,
    p.error_message,
    p.backup_path,
    p.created_at AS progress_created_at,
    p.updated_at AS progress_updated_at
FROM migration_job j
JOIN migration_volume_progress p ON p.job_id = j.id
ORDER BY j.created_at DESC, p.id;

-- name: GetPreference :one
SELECT * FROM user_preferences WHERE key = ?;

-- name: ListPreferences :many
SELECT * FROM user_preferences;

-- name: UpsertPreference :exec
INSERT INTO user_preferences (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

-- name: CreateStatsBatch :one
INSERT INTO stats_batch DEFAULT VALUES RETURNING *;

-- name: CreateStatsVolume :exec
INSERT INTO stats_volume (batch_id, volume_name, container_name, source_path, total_bytes, duration_ms)
VALUES (?, ?, ?, ?, ?, ?);

-- name: CreateStatsDrive :exec
INSERT INTO stats_drive (batch_id, mountpoint, device, total_bytes, used_bytes, free_bytes, duration_ms)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListStatsVolumeByName :many
SELECT * FROM stats_volume
WHERE volume_name = ? AND captured_at >= ? AND captured_at <= ?
ORDER BY captured_at;

-- name: ListStatsDriveByMountpoint :many
SELECT * FROM stats_drive
WHERE mountpoint = ? AND captured_at >= ? AND captured_at <= ?
ORDER BY captured_at;

-- name: ListStatsApplication :many
SELECT
    s.captured_at,
    s.container_name,
    SUM(s.total_bytes) AS total_bytes
FROM stats_volume s
WHERE s.container_name = ? AND s.captured_at >= ? AND s.captured_at <= ?
GROUP BY s.batch_id, s.container_name
ORDER BY s.captured_at;

-- name: DeleteOldStatsVolume :exec
DELETE FROM stats_volume WHERE captured_at < ?;

-- name: CreateMigrationLog :one
INSERT INTO migration_log (job_id, app_name, volume_name, source_path, source_drive, dest_path, dest_drive, total_bytes, duration_ms, status, error_message, started_at, completed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListMigrationLogs :many
SELECT * FROM migration_log
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListMigrationLogsByApp :many
SELECT * FROM migration_log
WHERE app_name = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListMigrationLogsByDrive :many
SELECT * FROM migration_log
WHERE dest_drive = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListMigrationLogsByStatus :many
SELECT * FROM migration_log
WHERE status = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: CountMigrationLogs :one
SELECT COUNT(*) FROM migration_log;

-- name: GetMigrationLogByJobID :many
SELECT * FROM migration_log WHERE job_id = ? ORDER BY id;

-- name: DeleteMigrationLog :exec
DELETE FROM migration_log WHERE id = ?;

-- name: DeleteOldStatsDrive :exec
DELETE FROM stats_drive WHERE captured_at < ?;

-- name: ListDistinctAppNames :many
SELECT DISTINCT app_name FROM migration_log ORDER BY app_name;

-- name: CreateUser :one
INSERT INTO users (email, username, password_hash, role, password_change_required)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ?;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = ?;

-- name: ListUsers :many
SELECT * FROM users ORDER BY username;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = ?;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = ?, password_change_required = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token, expires_at)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetRefreshToken :one
SELECT * FROM refresh_tokens WHERE token = ?;

-- name: DeleteRefreshToken :exec
DELETE FROM refresh_tokens WHERE token = ?;

-- name: DeleteUserRefreshTokens :exec
DELETE FROM refresh_tokens WHERE user_id = ?;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens WHERE expires_at < CURRENT_TIMESTAMP;

-- name: GetMigrationStats :one
SELECT
  CAST(COUNT(*) AS INTEGER) as total_count,
  CAST(COALESCE(SUM(CASE WHEN status = 'COMPLETED' THEN 1 ELSE 0 END), 0) AS INTEGER) as completed_count,
  CAST(COALESCE(SUM(CASE WHEN status = 'FAILED' THEN 1 ELSE 0 END), 0) AS INTEGER) as failed_count,
  CAST(COALESCE(SUM(total_bytes), 0) AS INTEGER) as total_bytes_moved,
  CAST(COALESCE(SUM(duration_ms), 0) AS INTEGER) as total_duration_ms,
  CAST(COUNT(DISTINCT app_name) AS INTEGER) as unique_apps
FROM migration_log;
