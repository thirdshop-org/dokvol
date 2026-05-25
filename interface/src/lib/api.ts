import type { DriveInfo, VolumeDetail, ApplicationVolumes, HealthCheckResponse, InitDriveResponse } from '$lib/types';

const BASE = '/api';

async function fetchJson<T>(path: string, options?: RequestInit): Promise<T> {
	const res = await fetch(`${BASE}${path}`, options);
	if (!res.ok) {
		throw new Error(`API ${path} failed: ${res.status} ${res.statusText}`);
	}
	return res.json();
}

export function getDrives(): Promise<DriveInfo[]> {
	return fetchJson<DriveInfo[]>('/drives');
}

export function getVolumes(): Promise<VolumeDetail[]> {
	return fetchJson<VolumeDetail[]>('/volumes');
}

export function getApplications(): Promise<ApplicationVolumes[]> {
	return fetchJson<ApplicationVolumes[]>('/applications');
}

export function checkDriveHealth(mountpoint: string): Promise<HealthCheckResponse> {
	return fetchJson<HealthCheckResponse>(`/drives/health?mountpoint=${encodeURIComponent(mountpoint)}`);
}

export function initDrive(mountpoint: string): Promise<InitDriveResponse> {
	return fetchJson<InitDriveResponse>('/drives/init', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ mountpoint }),
	});
}
