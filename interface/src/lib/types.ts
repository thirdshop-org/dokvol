export interface DriveInfo {
	device: string;
	mountpoint: string;
	fstype: string;
	total_gb: number;
	free_gb: number;
	used_pct: number;
}

export interface VolumeDetail {
	ContainerName: string;
	Type: string;
	Source: string;
	Destination: string;
	SystemDrive: DriveInfo | null;
}

export interface ApplicationVolumes {
	ContainerName: string;
	Volumes: VolumeDetail[];
}
