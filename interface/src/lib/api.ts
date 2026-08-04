import type { DriveInfo, VolumeDetail, ApplicationVolumes, HealthCheckResponse, InitDriveResponse, MigrateVolumeRequest, StartMigrationResponse, MigrationJob, APIError, SystemHealthResponse, StatsVolume, StatsDrive, StatsApplication, DeleteVolumeRequest, DeleteVolumeResponse, PreferencesResponse, HistoryListResponse, HistoryJobDetail, MigrationStats, VersionResponse, BrowseRequest, BrowseResponse, ReadFileRequest, ReadFileResponse, LoginRequest, CreateUserRequest, AuthResponse, RefreshRequest, RefreshResponse, ChangePasswordRequest, User, BackupTarget, BackupJob, BackupVolumeProgress, BackupSchedule, BackupListEntry, TrashEntry } from '$lib/types';
import { auth } from '$lib/stores/auth.svelte';

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
	const headers: Record<string, string> = {
		'Content-Type': 'application/json',
	};

	const token = auth.getAccessToken();
	if (token) {
		headers['Authorization'] = `Bearer ${token}`;
	}

	const opts: RequestInit = {
		...options,
		headers: { ...headers, ...((options?.headers as Record<string, string>) || {}) },
	};

	const res = await fetch(`${BASE}${path}`, opts);

	if (res.status === 401) {
		const rt = auth.getRefreshToken();
		if (rt) {
			try {
				const refreshRes = await fetch(`${BASE}/auth/refresh`, {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ refresh_token: rt } as RefreshRequest),
				});
				if (refreshRes.ok) {
					const data: RefreshResponse = await refreshRes.json();
					auth.updateTokens(data.access_token, data.refresh_token);
					headers['Authorization'] = `Bearer ${data.access_token}`;
					const retryRes = await fetch(`${BASE}${path}`, {
						...options,
						headers: { ...headers, ...((options?.headers as Record<string, string>) || {}) },
					});
					if (!retryRes.ok) {
						const body = await retryRes.json().catch(() => null) as APIError | null;
						if (body?.error_code) throw new ApiError(body);
						throw new Error(`API ${path} failed: ${retryRes.status} ${retryRes.statusText}`);
					}
					return retryRes.json();
				}
			} catch {
				// refresh failed
			}
		}
		auth.logout();
		if (typeof window !== 'undefined' && !window.location.pathname.startsWith('/login')) {
			window.location.href = '/login';
		}
		throw new ApiError({ error_code: 'AUTH.UNAUTHORIZED', message: 'Session expired' });
	}

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
		body: JSON.stringify({ mountpoint }),
	});
}

export function migrateVolume(req: MigrateVolumeRequest): Promise<StartMigrationResponse> {
	return fetchJson<StartMigrationResponse>('/volumes/migrate', {
		method: 'POST',
		body: JSON.stringify(req),
	});
}

export function getActiveMigrations(): Promise<MigrationJob[]> {
	return fetchJson<MigrationJob[]>('/volumes/migrate');
}

export function deleteVolumes(req: DeleteVolumeRequest): Promise<DeleteVolumeResponse> {
	return fetchJson<DeleteVolumeResponse>('/volumes', {
		method: 'DELETE',
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

export function updatePreference(key: string, value: string): Promise<{ key: string; value: string }> {
	return fetchJson<{ key: string; value: string }>('/preferences', {
		method: 'PUT',
		body: JSON.stringify({ key, value }),
	});
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
		body: JSON.stringify(req),
	});
}

export function readVolumeFile(req: ReadFileRequest): Promise<ReadFileResponse> {
	return fetchJson<ReadFileResponse>('/volumes/read-file', {
		method: 'POST',
		body: JSON.stringify(req),
	});
}

export function getTrash(signal?: AbortSignal): Promise<TrashEntry[]> {
	return fetchJson<TrashEntry[]>('/volumes/trash', { signal });
}

export function restoreTrashEntry(id: number): Promise<{ status: string }> {
	return fetchJson<{ status: string }>(`/volumes/trash/${id}/restore`, { method: 'POST' });
}

export function purgeTrashEntry(id: number): Promise<{ status: string }> {
	return fetchJson<{ status: string }>(`/volumes/trash/${id}`, { method: 'DELETE' });
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

export function login(req: LoginRequest): Promise<AuthResponse> {
	return fetchJson<AuthResponse>('/auth/login', {
		method: 'POST',
		body: JSON.stringify(req),
	});
}

export function logout(req: RefreshRequest): Promise<{ message: string }> {
	return fetchJson<{ message: string }>('/auth/logout', {
		method: 'POST',
		body: JSON.stringify(req),
	});
}

export function getProfile(): Promise<User> {
	return fetchJson<User>('/auth/me');
}

export function changePassword(req: ChangePasswordRequest): Promise<{ message: string }> {
	return fetchJson<{ message: string }>('/auth/change-password', {
		method: 'POST',
		body: JSON.stringify(req),
	});
}

export function getUsers(): Promise<User[]> {
	return fetchJson<User[]>('/auth/users');
}

export function createUser(req: CreateUserRequest): Promise<User> {
	return fetchJson<User>('/auth/users', {
		method: 'POST',
		body: JSON.stringify(req),
	});
}

export function deleteUser(id: number): Promise<{ status: string }> {
	return fetchJson<{ status: string }>(`/auth/users/${id}`, { method: 'DELETE' });
}

export function getBackupTargets(signal?: AbortSignal): Promise<BackupTarget[]> {
    return fetchJson<BackupTarget[]>('/backup/targets', { signal });
}

export function createBackupTarget(data: { name: string; provider: string; config: Record<string, unknown> }): Promise<{ id: string; name: string; provider: string; created_at: string }> {
    return fetchJson('/backup/targets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
    });
}

export function updateBackupTarget(id: string, data: { name: string; provider: string; config: Record<string, unknown> }): Promise<{ status: string }> {
    return fetchJson(`/backup/targets/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
    });
}

export function deleteBackupTarget(id: string): Promise<{ status: string }> {
    return fetchJson(`/backup/targets/${id}`, { method: 'DELETE' });
}

export function testBackupTarget(id: string): Promise<{ success: boolean; message: string }> {
    return fetchJson(`/backup/targets/${id}/test`, { method: 'POST' });
}

export function runBackup(appName: string, targetId: string): Promise<{ job_id: string; status: string }> {
    return fetchJson('/backup/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ app_name: appName, target_id: targetId }),
    });
}

export function getBackupJobs(params?: { limit?: number; offset?: number }): Promise<{ jobs: BackupJob[]; total: number }> {
    const search = new URLSearchParams();
    if (params?.limit) search.set('limit', String(params.limit));
    if (params?.offset) search.set('offset', String(params.offset));
    const qs = search.toString();
    return fetchJson(`/backup/jobs${qs ? '?' + qs : ''}`);
}

export function getBackupJob(id: string): Promise<{ id: string; status: string; volumes: BackupVolumeProgress[] }> {
    return fetchJson(`/backup/jobs/${id}`);
}

export function listBackupsOnTarget(targetId: string, appName: string): Promise<BackupListEntry[]> {
    return fetchJson(`/backup/targets/${targetId}/backups?app=${encodeURIComponent(appName)}`);
}

export function restoreBackup(data: { job_id: string; target_id: string; app_name: string; dest_mountpoint?: string }): Promise<{ job_id: string; app_name: string; status: string; volumes: { volume_name: string; dest_path: string; status: string; error?: string }[] }> {
    return fetchJson('/backup/restore', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
    });
}

export function getBackupSchedules(): Promise<BackupSchedule[]> {
    return fetchJson<BackupSchedule[]>('/backup/schedules');
}

export function createBackupSchedule(data: { target_id: string; app_name: string; cron_expr: string; retention: number }): Promise<{ id: string }> {
    return fetchJson('/backup/schedules', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
    });
}

export function updateBackupSchedule(id: string, data: { app_name?: string; cron_expr?: string; retention?: number; enabled?: boolean }): Promise<{ status: string }> {
    return fetchJson(`/backup/schedules/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
    });
}

export function deleteBackupSchedule(id: string): Promise<{ status: string }> {
    return fetchJson(`/backup/schedules/${id}`, { method: 'DELETE' });
}
