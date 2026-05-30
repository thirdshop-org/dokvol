import type { DriveInfo, VolumeDetail, ApplicationVolumes, HealthCheckResponse, InitDriveResponse, MigrateVolumeRequest, StartMigrationResponse, MigrationJob, APIError, SystemHealthResponse, StatsVolume, StatsDrive, StatsApplication } from '$lib/types';

const BASE = '/api';

export class ApiError extends Error {
	errorCode: string;
	details?: Record<string, unknown>;

	constructor(err: APIError) {
		super(err.message);
		this.name = 'ApiError';
		this.errorCode = err.error_code;
		this.details = err.details;
	}
}

async function fetchJson<T>(path: string, options?: RequestInit): Promise<T> {
	const res = await fetch(`${BASE}${path}`, options);
	if (!res.ok) {
		const body = await res.json().catch(() => null) as APIError | null;
		if (body?.error_code) {
			throw new ApiError(body);
		}
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

export function migrateVolume(req: MigrateVolumeRequest): Promise<StartMigrationResponse> {
	return fetchJson<StartMigrationResponse>('/volumes/migrate', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(req),
	});
}

export function getActiveMigrations(): Promise<MigrationJob[]> {
	return fetchJson<MigrationJob[]>('/volumes/migrate');
}

export function getMigrationStatus(jobId: string): Promise<MigrationJob> {
	return fetchJson<MigrationJob>(`/volumes/migrate/${jobId}`);
}

export function getSystemHealth(): Promise<SystemHealthResponse> {
	return fetchJson<SystemHealthResponse>('/health');
}

export function getStatsVolume(name: string, from?: string, to?: string): Promise<StatsVolume[]> {
	const params = new URLSearchParams({ name });
	if (from) params.set('from', from);
	if (to) params.set('to', to);
	return fetchJson<StatsVolume[]>(`/stats/volumes?${params}`);
}

export function getStatsDrive(mountpoint: string, from?: string, to?: string): Promise<StatsDrive[]> {
	const params = new URLSearchParams({ mountpoint });
	if (from) params.set('from', from);
	if (to) params.set('to', to);
	return fetchJson<StatsDrive[]>(`/stats/drives?${params}`);
}

export function getStatsApplication(name: string, from?: string, to?: string): Promise<StatsApplication[]> {
	const params = new URLSearchParams({ name });
	if (from) params.set('from', from);
	if (to) params.set('to', to);
	return fetchJson<StatsApplication[]>(`/stats/applications?${params}`);
}
