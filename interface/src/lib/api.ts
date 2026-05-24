import type { DriveInfo, VolumeDetail, ApplicationVolumes } from '$lib/types';

const BASE = '/api';

async function fetchJson<T>(path: string): Promise<T> {
	const res = await fetch(`${BASE}${path}`);
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
