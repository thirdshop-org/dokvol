export interface DriveInfo {
	device: string;
	mountpoint: string;
	fstype: string;
	total_gb: number;
	free_gb: number;
	used_pct: number;
}

export interface HealthCheckResponse {
	healthy: boolean;
	message?: string;
}

export interface InitDriveResponse {
	success: boolean;
	message?: string;
}

export interface VolumeDetail {
	ContainerName: string;
	Name: string;
	Type: string;
	Source: string;
	Destination: string;
	SystemDrive: DriveInfo | null;
	IsMigratable: boolean;
	MigratedDriveMountpoint?: string;
	MigratedDestPath?: string;
}

export interface ApplicationVolumes {
	ContainerName: string;
	Status: string;
	Volumes: VolumeDetail[];
}

export interface MigrateVolumeEntry {
	name: string;
	destination_mountpoint: string;
}

export interface MigrateVolumeRequest {
	application: string;
	destination_mountpoint?: string;
	volumes?: MigrateVolumeEntry[];
}

export interface MigrateVolumeResponse {
	success: boolean;
	message?: string;
}

export interface APIError {
	error_code: string;
	message: string;
	details?: Record<string, unknown>;
}

export interface VolumeProgress {
	volume_name: string;
	step: string;
	total_bytes: number;
	transferred_bytes: number;
	error?: string;
	created_at: string;
	updated_at: string;
}

export interface MigrationJob {
	id: string;
	app_name: string;
	status: 'pending' | 'running' | 'completed' | 'failed';
	created_at: string;
	updated_at: string;
	completed_at: string;
	volumes: VolumeProgress[];
}

export interface DeleteVolumeEntry {
	name: string;
	source: string;
	type: string;
}

export interface DeleteVolumeRequest {
	volumes: DeleteVolumeEntry[];
}

export interface DeleteVolumeResponse {
	success: boolean;
	errors?: string[];
}

export interface StartMigrationResponse {
	job_id: string;
	status: string;
}

export interface VersionResponse {
	version: string;
}

export interface SystemHealthResponse {
	healthy: boolean;
	checks?: { check: string; passed: boolean; error?: string }[];
}

export interface StatsVolume {
	id: number;
	batch_id: number;
	volume_name: string;
	container_name: string;
	source_path: string;
	total_bytes: number;
	duration_ms: number;
	captured_at: string;
}

export interface StatsDrive {
	id: number;
	batch_id: number;
	mountpoint: string;
	device: string;
	total_bytes: number;
	used_bytes: number;
	free_bytes: number;
	duration_ms: number;
	captured_at: string;
}

export interface StatsApplication {
	captured_at: string;
	container_name: string;
	total_bytes: number | null;
}

export type PreferencesResponse = Record<string, string>;

export interface MigrationStats {
	total_count: number;
	completed_count: number;
	failed_count: number;
	total_bytes_moved: number;
	total_duration_ms: number;
	unique_apps: number;
}

export interface MigrationLogEntry {
	id: number;
	job_id: string;
	app_name: string;
	volume_name: string;
	source_path: string;
	source_drive?: string;
	dest_path: string;
	dest_drive: string;
	total_bytes: number;
	duration_ms: number;
	status: string;
	error_message?: string;
	started_at?: string;
	completed_at?: string;
	created_at: string;
}

export interface HistoryListResponse {
	entries: MigrationLogEntry[];
	total: number;
}

export interface HistoryJobDetail {
	job_id: string;
	app_name: string;
	status: string;
	started_at?: string;
	completed_at?: string;
	volumes: MigrationLogEntry[];
}

export interface BrowseRequest {
	container: string;
	path: string;
}

export interface FileEntry {
	name: string;
	is_dir: boolean;
	size: number;
	mode: string;
	mod_time: string;
}

export interface BrowseResponse {
	entries: FileEntry[];
	path: string;
}

export interface ReadFileRequest {
	container: string;
	path: string;
}

export interface ReadFileResponse {
	content: string;
	truncated: boolean;
	binary: boolean;
	size: number;
}

export interface User {
	id: number;
	email: string;
	username: string;
	role: 'admin' | 'user';
	password_change_required: boolean;
	created_at: string;
}

export interface LoginRequest {
	email: string;
	password: string;
}

export interface RegisterRequest {
	email: string;
	username: string;
	password: string;
}

export interface AuthResponse {
	access_token: string;
	refresh_token: string;
	user: User;
}

export interface RefreshRequest {
	refresh_token: string;
}

export interface RefreshResponse {
	access_token: string;
	refresh_token: string;
}

export interface ChangePasswordRequest {
	old_password: string;
	new_password: string;
}

export interface BackupTarget {
    id: string;
    name: string;
    provider: 's3' | 'sftp' | 'local';
    created_at: string;
    updated_at: string;
}

export interface BackupJob {
    id: string;
    target_id: string;
    app_name: string;
    status: string;
    total_bytes: number;
    duration_ms: number;
    error_message?: string;
    started_at: string;
    completed_at: string;
}

export interface BackupVolumeProgress {
    VolumeName: string;
    SourcePath: string;
    BackupPath: string;
    Status: string;
    TotalBytes: number;
    TransferredBytes: number;
    ErrorMessage: string;
}

export interface BackupSchedule {
    id: string;
    target_id: string;
    app_name: string;
    cron_expr: string;
    retention: number;
    enabled: boolean;
}

export interface BackupListEntry {
    path: string;
    name: string;
    size: number;
    modified_at: string;
}
