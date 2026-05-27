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
