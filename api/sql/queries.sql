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
