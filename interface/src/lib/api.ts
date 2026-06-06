import type { DriveInfo, VolumeDetail, ApplicationVolumes, HealthCheckResponse, InitDriveResponse, MigrateVolumeRequest, StartMigrationResponse, MigrationJob, APIError, SystemHealthResponse, StatsVolume, StatsDrive, StatsApplication, DeleteVolumeRequest, DeleteVolumeResponse, PreferencesResponse, HistoryListResponse, HistoryJobDetail, MigrationStats, VersionResponse, BrowseRequest, BrowseResponse, ReadFileRequest, ReadFileResponse } from '$lib/types';

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

async function fetchJson<T>(path: string, options?: RequestInit & { signal?: AbortSignal }): Promise<T> {
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

export function getDrives(signal?: AbortSignal): Promise<DriveInfo[]> {
	return fetchJson<DriveInfo[]>('/drives', { signal });
}

export function getVolumes(signal?: AbortSignal): Promise<VolumeDetail[]> {
	return fetchJson<VolumeDetail[]>('/volumes', { signal });
}

export function getApplications(signal?: AbortSignal): Promise<ApplicationVolumes[]> {
	return fetchJson<ApplicationVolumes[]>('/applications', { signal });
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

export function deleteVolumes(req: DeleteVolumeRequest): Promise<DeleteVolumeResponse> {
	return fetchJson<DeleteVolumeResponse>('/volumes', {
		method: 'DELETE',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(req),
	});
}

export function getMigrationStatus(jobId: string): Promise<MigrationJob> {
	return fetchJson<MigrationJob>(`/volumes/migrate/${jobId}`);
}

export function getVersion(): Promise<VersionResponse> {
	return fetchJson<VersionResponse>('/version');
}

export function getSystemHealth(): Promise<SystemHealthResponse> {
	return fetchJson<SystemHealthResponse>('/health');
}

export function getStatsVolume(name: string, from?: string, to?: string, signal?: AbortSignal): Promise<StatsVolume[]> {
	const params = new URLSearchParams({ name });
	if (from) params.set('from', from);
	if (to) params.set('to', to);
	return fetchJson<StatsVolume[]>(`/stats/volumes?${params}`, { signal });
}

export function getStatsDrive(mountpoint: string, from?: string, to?: string, signal?: AbortSignal): Promise<StatsDrive[]> {
	const params = new URLSearchParams({ mountpoint });
	if (from) params.set('from', from);
	if (to) params.set('to', to);
	return fetchJson<StatsDrive[]>(`/stats/drives?${params}`, { signal });
}

export function getPreferences(): Promise<PreferencesResponse> {
	return fetchJson<PreferencesResponse>('/preferences');
}

export function getStatsApplication(name: string, from?: string, to?: string, signal?: AbortSignal): Promise<StatsApplication[]> {
	const params = new URLSearchParams({ name });
	if (from) params.set('from', from);
	if (to) params.set('to', to);
	return fetchJson<StatsApplication[]>(`/stats/applications?${params}`, { signal });
}

export function getHistory(params?: { limit?: number; offset?: number; app?: string; drive?: string; status?: string }): Promise<HistoryListResponse> {
	const search = new URLSearchParams();
	if (params?.limit) search.set('limit', String(params.limit));
	if (params?.offset) search.set('offset', String(params.offset));
	if (params?.app) search.set('app', params.app);
	if (params?.drive) search.set('drive', params.drive);
	if (params?.status) search.set('status', params.status);
	const qs = search.toString();
	return fetchJson<HistoryListResponse>(`/history${qs ? '?' + qs : ''}`);
}

export function getStatsMigration(signal?: AbortSignal): Promise<MigrationStats> {
	return fetchJson<MigrationStats>('/stats/migrations', { signal });
}

export function getHistoryAppNames(): Promise<string[]> {
	return fetchJson<string[]>('/history/names');
}

export function getHistoryJob(jobId: string): Promise<HistoryJobDetail> {
	return fetchJson<HistoryJobDetail>(`/history/${jobId}`);
}

export function browseVolume(req: BrowseRequest): Promise<BrowseResponse> {
	return fetchJson<BrowseResponse>('/volumes/browse', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(req),
	});
}

export function readVolumeFile(req: ReadFileRequest): Promise<ReadFileResponse> {
	return fetchJson<ReadFileResponse>('/volumes/read-file', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(req),
	});
}

export function rescanHistory(): Promise<{ success: boolean }> {
	return fetchJson<{ success: boolean }>('/history/rescan', { method: 'POST' });
}

export function stopApplication(name: string, signal?: AbortSignal): Promise<{ status: string }> {
	return fetchJson<{ status: string }>(`/applications/${encodeURIComponent(name)}/stop`, { method: 'POST', signal });
}

export function startApplication(name: string, signal?: AbortSignal): Promise<{ status: string }> {
	return fetchJson<{ status: string }>(`/applications/${encodeURIComponent(name)}/start`, { method: 'POST', signal });
}

export function restartApplication(name: string, signal?: AbortSignal): Promise<{ status: string }> {
	return fetchJson<{ status: string }>(`/applications/${encodeURIComponent(name)}/restart`, { method: 'POST', signal });
}
