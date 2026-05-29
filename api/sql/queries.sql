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
    p.created_at AS progress_created_at,
    p.updated_at AS progress_updated_at
FROM migration_job j
JOIN migration_volume_progress p ON p.job_id = j.id
ORDER BY j.created_at DESC, p.id;
