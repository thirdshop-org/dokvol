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
}

export interface ApplicationVolumes {
	ContainerName: string;
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
}

export interface MigrationJob {
	id: string;
	app_name: string;
	status: 'pending' | 'running' | 'completed' | 'failed';
	volumes: VolumeProgress[];
}

export interface StartMigrationResponse {
	job_id: string;
	status: string;
}

export interface SystemHealthResponse {
	healthy: boolean;
	checks?: { check: string; passed: boolean; error?: string }[];
}
