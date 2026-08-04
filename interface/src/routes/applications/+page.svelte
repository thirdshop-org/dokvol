<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { getApplications, getDrives, migrateVolume, getActiveMigrations, getMigrationStatus, getStatsApplication, getStatsVolume, stopApplication, startApplication, restartApplication } from '$lib/api';
	import { errorMessage } from '$lib/utils/errors';
	import { t } from '$lib/i18n';
	import type { ApplicationVolumes, DriveInfo, VolumeDetail, MigrationJob, VolumeProgress, StatsApplication, StatsVolume } from '$lib/types';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { LoaderCircle, ArrowUpFromLine, History, Square, Play, RotateCcw, Search } from '@lucide/svelte';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Table from '$lib/components/ui/table/index.js';
	import * as Checkbox from '$lib/components/ui/checkbox/index.js';
	import { formatBytes } from '$lib/utils/format';

	let apps = $state<ApplicationVolumes[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let search = $state('');

	let appStats = $state<Record<string, StatsApplication | null>>({});
	let volumeStats = $state<Record<string, StatsVolume | null>>({});

	let viewMode = $state<'normal' | 'bydrive'>('normal');

	type DriveGroup = {
		drive: DriveInfo;
		volumes: (VolumeDetail & { appName: string })[];
		totalBytes: number;
	};

	const driveGroups = $derived.by(() => {
		const map = new Map<string, DriveGroup>();
		for (const app of filteredApps) {
			const cleanName = app.ContainerName.replace(/^\//, '');
			for (const vol of app.Volumes) {
				const key = vol.SystemDrive?.mountpoint ?? '';
				if (!map.has(key)) {
					const fallback: DriveInfo = { device: '?', mountpoint: key, fstype: '', total_gb: 0, free_gb: 0, used_pct: 0 };
					map.set(key, { drive: vol.SystemDrive ?? fallback, volumes: [], totalBytes: 0 });
				}
				const group = map.get(key)!;
				group.volumes.push({ ...vol, appName: cleanName });
				const volKey = vol.Name || vol.Source.split('/').filter(Boolean).pop() || '';
				group.totalBytes += volumeStats[volKey]?.total_bytes ?? 0;
			}
		}
		return [...map.values()].sort((a, b) => a.drive.mountpoint.localeCompare(b.drive.mountpoint));
	});

	let filteredApps = $derived(
		search
			? apps.filter(a => {
				const q = search.toLowerCase();
				if (a.ContainerName.replace(/^\//, '').toLowerCase().includes(q)) return true;
				return a.Volumes.some(v =>
					v.Type.toLowerCase().includes(q) ||
					v.Source.toLowerCase().includes(q) ||
					v.Destination.toLowerCase().includes(q)
				);
			})
			: apps
	);

	let modalOpen = $state(false);
	let selectedApp = $state<ApplicationVolumes | null>(null);
	let drives = $state<DriveInfo[]>([]);
	let migrating = $state(false);
	let resultMessage = $state<string | null>(null);

	let currentJobId = $state<string | null>(null);
	let currentJob = $state<MigrationJob | null>(null);
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	let activeJobs = $state<MigrationJob[]>([]);
	let activePollTimer: ReturnType<typeof setInterval> | null = null;

	let historyModalOpen = $state(false);
	let historyAppName = $state<string | null>(null);
	let historyJobs = $state<MigrationJob[]>([]);

	let actionLoading = $state<Record<string, string | null>>({});

	let confirmAction = $state<{ appName: string; action: 'stop' | 'restart' } | null>(null);
	let confirmOpen = $state(false);

	async function handleAction(appName: string, action: 'stop' | 'start' | 'restart') {
		if (action === 'stop' || action === 'restart') {
			confirmAction = { appName, action };
			confirmOpen = true;
			return;
		}
		await executeAction(appName, action);
	}

	async function executeAction(appName: string, action: 'stop' | 'start' | 'restart') {
		actionLoading[appName] = action;
		actionLoading = actionLoading;
		try {
			if (action === 'stop') await stopApplication(appName);
			else if (action === 'start') await startApplication(appName);
			else await restartApplication(appName);
			apps = await getApplications();
		} catch {
			// Error handled below via the loading state remaining
		} finally {
			actionLoading[appName] = null;
			actionLoading = actionLoading;
			confirmAction = null;
			confirmOpen = false;
		}
	}

	$effect(() => {
		if (!modalOpen) {
			stopPoll();
		}
	});

	type VolumeRow = {
		name: string;
		source: string;
		IsMigratable: boolean;
		checked: boolean;
		destination: string;
	};

	let volumes = $state<VolumeRow[]>([]);
	let mode = $state<'same' | 'individual'>('same');
	let sameDest = $state<string>('');

	onMount(async () => {
		try {
			apps = await getApplications();
		} catch (e) {
			error = errorMessage(e);
		} finally {
			loading = false;
		}
		fetchStats();
		startActivePoll();
	});

	async function fetchStats() {
		const appNames = apps.map(a => a.ContainerName);
		const appResults = await Promise.allSettled(
			appNames.map(name =>
				getStatsApplication(name).then(stats => ({ name, stats: stats.at(-1) ?? null }))
			)
		);
		const appMap: Record<string, StatsApplication | null> = {};
		for (const r of appResults) {
			if (r.status === 'fulfilled') appMap[r.value.name] = r.value.stats;
		}
		appStats = appMap;

		const volumeNames = new Set<string>();
		for (const app of apps) {
			for (const vol of app.Volumes) {
				const key = vol.Name || vol.Source.split('/').filter(Boolean).pop() || '';
				if (key) volumeNames.add(key);
			}
		}
		const volResults = await Promise.allSettled(
			[...volumeNames].map(name =>
				getStatsVolume(name).then(stats => ({ name, stats: stats.at(-1) ?? null }))
			)
		);
		const volMap: Record<string, StatsVolume | null> = {};
		for (const r of volResults) {
			if (r.status === 'fulfilled') volMap[r.value.name] = r.value.stats;
		}
		volumeStats = volMap;
	}

	onDestroy(() => {
		stopPoll();
		stopActivePoll();
	});

	function startActivePoll() {
		activePollTimer = setInterval(async () => {
			try {
				const all = await getActiveMigrations();
				activeJobs = all.filter(j => j.status === 'running' || j.status === 'pending');
			} catch {
				// ignore
			}
		}, 3000);
	}

	function stopActivePoll() {
		if (activePollTimer) {
			clearInterval(activePollTimer);
			activePollTimer = null;
		}
	}

	function startJobPoll(jobId: string) {
		stopPoll();
		pollTimer = setInterval(async () => {
			try {
				const job = await getMigrationStatus(jobId);
				currentJob = job;
				if (job.status === 'completed' || job.status === 'failed') {
					stopPoll();
					if (job.status === 'completed') {
						resultMessage = $t('applications.migration.success');
						apps = await getApplications();
						fetchStats();
					} else {
						const failed = job.volumes.find(v => v.step === 'failed');
						resultMessage = failed?.error || $t('error.default');
					}
				}
			} catch {
				stopPoll();
			}
		}, 2000);
	}

	function stopPoll() {
		if (pollTimer) {
			clearInterval(pollTimer);
			pollTimer = null;
		}
	}

	function hasActiveJob(appName: string): boolean {
		return activeJobs.some(j => j.app_name === appName);
	}

	async function openHistory(appName: string) {
		historyAppName = appName;
		try {
			const all = await getActiveMigrations();
			historyJobs = all.filter(j => j.app_name === appName);
		} catch {
			historyJobs = [];
		}
		historyModalOpen = true;
	}

	function openModal(app: ApplicationVolumes) {
		selectedApp = app;
		resultMessage = null;
		currentJobId = null;
		currentJob = null;
		migrating = false;
		mode = 'same';
		sameDest = '';
		volumes = app.Volumes.map((v: VolumeDetail) => ({
			name: v.Name || v.Source.split('/').pop() || 'unknown',
			source: v.Source,
			IsMigratable: v.IsMigratable,
			checked: v.IsMigratable,
			destination: '',
		}));
		getDrives().then((d) => (drives = d));
		modalOpen = true;
	}

	function handleModalClose() {
		stopPoll();
		currentJobId = null;
		currentJob = null;
		modalOpen = false;
	}

	function handleModeChange(newMode: 'same' | 'individual') {
		mode = newMode;
		if (newMode === 'same' && sameDest) {
			for (const v of volumes) {
				v.destination = sameDest;
			}
		}
	}

	function handleSameDestChange(mountpoint: string) {
		sameDest = mountpoint;
		for (const v of volumes) {
			v.destination = mountpoint;
		}
	}

	function handleRowDestChange(index: number, mountpoint: string) {
		volumes[index].destination = mountpoint;
		if (mode === 'same') {
			sameDest = mountpoint;
			for (const v of volumes) {
				v.destination = mountpoint;
			}
		} else {
			const checked = volumes.filter((v) => v.checked);
			if (checked.length > 1) {
				for (const v of checked) {
					v.destination = mountpoint;
				}
			}
		}
	}

	function allChecked() {
		const checkable = volumes.filter(v => v.IsMigratable);
		return checkable.length > 0 && checkable.every((v) => v.checked);
	}

	function toggleAll() {
		const newVal = !allChecked();
		for (const v of volumes) {
			if (v.IsMigratable) v.checked = newVal;
		}
	}

	function stepLabel(step: string): string {
		const key = `step.${step}`;
		const label = $t(key);
		return label !== key ? label : step;
	}

	function progressPct(vol: VolumeProgress): number {
		if (vol.total_bytes <= 0) return 0;
		return Math.round((vol.transferred_bytes / vol.total_bytes) * 100);
	}

	async function handleMigrate() {
		if (!selectedApp) return;
		const appName = selectedApp.ContainerName;

		if (mode === 'same') {
			if (!sameDest) {
				resultMessage = $t('applications.error.noDest');
				return;
			}
		} else {
			const selected = volumes.filter((v) => v.checked && v.destination);
			if (selected.length === 0) {
				resultMessage = $t('applications.error.noVolume');
				return;
			}
		}

		migrating = true;
		resultMessage = null;

		try {
			let jobId: string;
			if (mode === 'same') {
				const res = await migrateVolume({
					application: appName,
					destination_mountpoint: sameDest,
				});
				jobId = res.job_id;
			} else {
				const selected = volumes.filter((v) => v.checked && v.destination);
				const res = await migrateVolume({
					application: appName,
					volumes: selected.map((v) => ({
						name: v.name,
						destination_mountpoint: v.destination,
					})),
				});
				jobId = res.job_id;
			}
			currentJobId = jobId;
			startJobPoll(jobId);
		} catch (e) {
			resultMessage = errorMessage(e);
			migrating = false;
		}
	}

	function statusLabel(state: string): string {
		try {
			return $t(`container.status.${state}`);
		} catch {
			return state;
		}
	}

	function statusClass(state: string): string {
		switch (state) {
			case 'running': return 'bg-green-100 text-green-800 hover:bg-green-100 dark:bg-green-900 dark:text-green-100 dark:hover:bg-green-900';
			case 'exited': return 'bg-red-100 text-red-800 hover:bg-red-100 dark:bg-red-900 dark:text-red-100 dark:hover:bg-red-900';
			case 'paused': return 'bg-yellow-100 text-yellow-800 hover:bg-yellow-100 dark:bg-yellow-900 dark:text-yellow-100 dark:hover:bg-yellow-900';
			case 'restarting': return 'bg-blue-100 text-blue-800 hover:bg-blue-100 dark:bg-blue-900 dark:text-blue-100 dark:hover:bg-blue-900';
			case 'removing': case 'dead': return 'bg-purple-100 text-purple-800 hover:bg-purple-100 dark:bg-purple-900 dark:text-purple-100 dark:hover:bg-purple-900';
			default: return 'bg-gray-100 text-gray-800 hover:bg-gray-100 dark:bg-gray-800 dark:text-gray-100 dark:hover:bg-gray-800';
		}
	}

</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight">{$t('applications.title')}</h1>
			<p class="text-muted-foreground">{$t('applications.description')}</p>
		</div>
		<div class="flex items-center gap-1 rounded-lg border p-0.5">
			<button
				class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
				class:bg-muted={viewMode === 'normal'}
				class:text-muted-foreground={viewMode !== 'normal'}
				onclick={() => (viewMode = 'normal')}
			>{$t('applications.viewModes.normal')}</button>
			<button
				class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
				class:bg-muted={viewMode === 'bydrive'}
				class:text-muted-foreground={viewMode !== 'bydrive'}
				onclick={() => (viewMode = 'bydrive')}
			>{$t('applications.viewModes.byDrive')}</button>
		</div>
	</div>

	{#if loading}
		<p class="text-muted-foreground">{$t('applications.loading')}</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else}
		<div class="relative">
			<Search class="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
			<Input bind:value={search} placeholder={$t('applications.search')} class="pl-9" />
		</div>

		{#if viewMode === 'normal'}
			{#each filteredApps as app (app.ContainerName)}
			{@const busy = hasActiveJob(app.ContainerName)}
			<div class="rounded-lg border" class:border-primary={busy}>
				<div class="flex items-center justify-between border-b bg-muted/30 px-4 py-3">
					<h2 class="font-semibold">
						<a href="/applications/{app.ContainerName.replace(/^\//, '')}" class="hover:underline">{app.ContainerName.replace(/^\//, '')}</a>
						<Badge class={statusClass(app.Status)}>{statusLabel(app.Status)}</Badge>
						{#if busy}
							<span class="ml-2 inline-flex items-center gap-1 text-xs text-primary">
								<LoaderCircle class="size-3 animate-spin" />
								{$t('applications.migration.running')}
							</span>
						{/if}
						{#if appStats[app.ContainerName]?.total_bytes != null}
							<span class="ml-2 text-xs font-normal text-muted-foreground">
								({$t('applications.volumeCount', { n: app.Volumes.length })} — {formatBytes(appStats[app.ContainerName]!.total_bytes!)})
							</span>
						{:else}
							<span class="ml-2 text-xs font-normal text-muted-foreground">
								({$t('applications.volumeCount', { n: app.Volumes.length })})
							</span>
						{/if}
					</h2>
					<div class="flex gap-2">
						{#if app.Status === 'running'}
							<Button size="sm" variant="outline" onclick={() => handleAction(app.ContainerName, 'stop')} disabled={busy || actionLoading[app.ContainerName] !== null}>
								{#if actionLoading[app.ContainerName] === 'stop'}
									<LoaderCircle class="size-3.5 animate-spin" />
								{:else}
									<Square class="size-3.5" />
								{/if}
								{$t('applications.stop')}
							</Button>
							<Button size="sm" variant="outline" onclick={() => handleAction(app.ContainerName, 'restart')} disabled={busy || actionLoading[app.ContainerName] !== null}>
								{#if actionLoading[app.ContainerName] === 'restart'}
									<LoaderCircle class="size-3.5 animate-spin" />
								{:else}
									<RotateCcw class="size-3.5" />
								{/if}
								{$t('applications.restart')}
							</Button>
						{:else}
							<Button size="sm" variant="outline" onclick={() => handleAction(app.ContainerName, 'start')} disabled={busy || actionLoading[app.ContainerName] !== null}>
								{#if actionLoading[app.ContainerName] === 'start'}
									<LoaderCircle class="size-3.5 animate-spin" />
								{:else}
									<Play class="size-3.5" />
								{/if}
								{$t('applications.start')}
							</Button>
						{/if}
						<Button size="sm" variant="outline" onclick={() => openHistory(app.ContainerName)}>
							<History class="size-3.5" />
							{$t('applications.history')}
						</Button>
						<Button size="sm" onclick={() => openModal(app)} disabled={busy}>
							<ArrowUpFromLine class="size-3.5" />
							{$t('applications.migrate')}
						</Button>
					</div>
				</div>
				<table class="w-full text-sm">
					<thead class="border-b text-muted-foreground">
						<tr>
							<th class="px-4 py-2 text-left font-medium">Type</th>
							<th class="px-4 py-2 text-left font-medium">Size</th>
							<th class="px-4 py-2 text-left font-medium">Source</th>
							<th class="px-4 py-2 text-left font-medium">Drive</th>
							<th class="px-4 py-2 text-left font-medium">Destination</th>
						</tr>
					</thead>
					<tbody>
						{#each app.Volumes as vol, i (i)}
							{@const volKey = vol.Name || vol.Source.split('/').filter(Boolean).pop() || ''}
							{@const vs = volumeStats[volKey]}
							<tr class="border-b last:border-0 hover:bg-muted/30">
								<td class="px-4 py-2">
									{#if !vol.IsMigratable}
										<Badge class="bg-purple-100 text-purple-800 hover:bg-purple-100 dark:bg-purple-900 dark:text-purple-100 dark:hover:bg-purple-900">{vol.Type} — {$t('applications.inMemory')}</Badge>
									{:else}
										<Badge class={vol.Type === 'volume' ? 'bg-blue-100 text-blue-800 hover:bg-blue-100 dark:bg-blue-900 dark:text-blue-100 dark:hover:bg-blue-900' : 'bg-yellow-100 text-yellow-800 hover:bg-yellow-100 dark:bg-yellow-900 dark:text-yellow-100 dark:hover:bg-yellow-900'}>{vol.Type}</Badge>
									{/if}
								</td>
								<td class="px-4 py-2 font-mono text-xs text-muted-foreground">
									{vs?.total_bytes != null ? formatBytes(vs.total_bytes) : '—'}
								</td>
								<td class="px-4 py-2 font-mono text-xs">
									{vol.Source}
									{#if vol.MigratedDriveMountpoint}
										<Badge class="bg-emerald-100 text-emerald-800 hover:bg-emerald-100 dark:bg-emerald-900 dark:text-emerald-100 dark:hover:bg-emerald-900 ml-1 text-[10px]">→ {vol.MigratedDriveMountpoint}</Badge>
									{/if}
								</td>
								<td class="px-4 py-2 font-mono text-xs">
									{#if vol.SystemDrive}
										<Tooltip.Root>
											<Tooltip.Trigger>
												<Badge class="bg-amber-100 text-amber-800 hover:bg-amber-100 dark:bg-amber-900 dark:text-amber-100 dark:hover:bg-amber-900 text-[10px] cursor-default">{vol.SystemDrive.device}</Badge>
											</Tooltip.Trigger>
											<Tooltip.Content side="top" align="center">
												{vol.SystemDrive.mountpoint}
											</Tooltip.Content>
										</Tooltip.Root>
									{:else}
										<span class="text-muted-foreground">—</span>
									{/if}
								</td>
								<td class="px-4 py-2 font-mono text-xs">{vol.Destination}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/each}
		{:else if viewMode === 'bydrive'}
			{#each driveGroups as group (group.drive.mountpoint)}
				<div class="rounded-lg border">
					<div class="flex items-center justify-between border-b bg-muted/30 px-4 py-3">
						<h2 class="flex items-center gap-2 font-semibold">
							<Badge class="bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-100 text-[10px]">{group.drive.device}</Badge>
							<span class="text-xs font-normal text-muted-foreground">{group.drive.mountpoint}</span>
							{#if group.totalBytes > 0}
								<span class="text-xs font-normal text-muted-foreground">— {formatBytes(group.totalBytes)}</span>
							{/if}
						</h2>
						<span class="text-xs text-muted-foreground">{$t('applications.volumeCount', { n: group.volumes.length })}</span>
					</div>
					<table class="w-full text-sm">
						<thead class="border-b text-muted-foreground">
							<tr>
								<th class="px-4 py-2 text-left font-medium">App</th>
								<th class="px-4 py-2 text-left font-medium">Type</th>
								<th class="px-4 py-2 text-left font-medium">Size</th>
								<th class="px-4 py-2 text-left font-medium">Source</th>
								<th class="px-4 py-2 text-left font-medium">Destination</th>
							</tr>
						</thead>
						<tbody>
							{#each group.volumes as vol, i (i)}
								{@const volKey = vol.Name || vol.Source.split('/').filter(Boolean).pop() || ''}
								{@const vs = volumeStats[volKey]}
								<tr class="border-b last:border-0 hover:bg-muted/30">
									<td class="px-4 py-2 font-mono text-xs">
										<a href="/applications/{vol.appName}" class="hover:underline font-medium">{vol.appName}</a>
									</td>
									<td class="px-4 py-2">
										<Badge class={vol.Type === 'volume' ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-100' : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-100'}>{vol.Type}</Badge>
									</td>
									<td class="px-4 py-2 font-mono text-xs text-muted-foreground">{vs?.total_bytes != null ? formatBytes(vs.total_bytes) : '—'}</td>
									<td class="px-4 py-2 font-mono text-xs max-w-48 truncate" title={vol.Source}>{vol.Source}</td>
									<td class="px-4 py-2 font-mono text-xs">{vol.Destination}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/each}
		{/if}
	{/if}
</div>

<!-- Modale d'historique -->
<Dialog.Root bind:open={historyModalOpen}>
	<Dialog.Content class="sm:max-w-lg">
		<Dialog.Header>
			<Dialog.Title>
				{$t('applications.migration.title', { name: historyAppName?.replace(/^\//, '') ?? '' })} — {$t('applications.history')}
			</Dialog.Title>
		</Dialog.Header>
		<div class="space-y-3 max-h-96 overflow-y-auto">
			{#if historyJobs.length === 0}
				<p class="text-sm text-muted-foreground">{$t('applications.migration.noHistory')}</p>
			{:else}
				{#each historyJobs as job (job.id)}
					<div class="rounded-lg border p-3 text-sm">
						<div class="flex items-center justify-between mb-2">
							<Badge variant={job.status === 'completed' ? 'success' : job.status === 'failed' ? 'destructive' : 'running'}>{job.status}</Badge>
							<span class="text-xs text-muted-foreground font-mono">{job.id.slice(0, 8)}…</span>
						</div>
						{#each job.volumes as vol}
							<div class="flex items-center justify-between py-1 text-xs border-b last:border-0 border-muted/30">
								<div class="flex items-center gap-2">
									<span class="font-medium">{vol.volume_name}</span>
									<span class="text-muted-foreground">— {stepLabel(vol.step)}</span>
								</div>
								<div class="flex items-center gap-2">
									{#if vol.total_bytes > 0}
										<span class="text-muted-foreground">{formatBytes(vol.total_bytes)}</span>
									{/if}
									{#if vol.step === 'failed' && vol.error}
										<span class="text-destructive text-xs max-w-48 truncate" title={vol.error}>{vol.error}</span>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				{/each}
			{/if}
		</div>
		<Dialog.Footer>
			<Button onclick={() => (historyModalOpen = false)}>{$t('applications.migration.close')}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Confirmation Stop / Restart -->
<Dialog.Root bind:open={confirmOpen}>
	<Dialog.Content class="sm:max-w-sm">
		<Dialog.Header>
			<Dialog.Title>
				{confirmAction?.action === 'stop' ? $t('applications.confirm.stopTitle') : $t('applications.confirm.restartTitle')}
			</Dialog.Title>
			<Dialog.Description>
				{confirmAction?.action === 'stop' ? $t('applications.confirm.stopDesc', { name: confirmAction?.appName?.replace(/^\//, '') ?? '' }) : $t('applications.confirm.restartDesc', { name: confirmAction?.appName?.replace(/^\//, '') ?? '' })}
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer class="flex gap-2">
			<Button variant="outline" onclick={() => { confirmAction = null; confirmOpen = false; }}>
				{$t('applications.confirm.cancel')}
			</Button>
			<Button
				variant={confirmAction?.action === 'stop' ? 'destructive' : 'default'}
				onclick={() => confirmAction && executeAction(confirmAction.appName, confirmAction.action)}
			>
				{confirmAction?.action === 'stop' ? $t('applications.stop') : $t('applications.restart')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Modale de migration -->
<Dialog.Root bind:open={modalOpen}>
	<Dialog.Content class="sm:max-w-2xl">
		<Dialog.Header>
			<Dialog.Title>
				{$t('applications.migration.title', { name: selectedApp?.ContainerName.replace(/^\//, '') ?? '' })}
			</Dialog.Title>
			<Dialog.Description>
				{$t('applications.migration.description')}
			</Dialog.Description>
		</Dialog.Header>

		<div class="space-y-4">
			{#if currentJobId && currentJob}
				<div class="space-y-3">
					{#each currentJob.volumes as vol}
						<div>
							<div class="flex items-center justify-between text-sm">
								<span class="font-medium">{vol.volume_name}</span>
								<span class="text-xs text-muted-foreground">{stepLabel(vol.step)}</span>
							</div>
							<div class="mt-1 h-2 w-full overflow-hidden rounded-full bg-muted">
								<div
									class="h-full rounded-full transition-all"
									class:bg-primary={vol.step !== 'completed' && vol.step !== 'failed'}
									class:bg-green-500={vol.step === 'completed'}
									class:bg-destructive={vol.step === 'failed'}
									style="width: {progressPct(vol)}%"
								></div>
							</div>
							{#if vol.total_bytes > 0}
								<p class="mt-0.5 text-xs text-muted-foreground">
									{formatBytes(vol.transferred_bytes)} / {formatBytes(vol.total_bytes)}
								</p>
							{/if}
							{#if vol.error}
								<p class="mt-0.5 text-xs text-destructive">{vol.error}</p>
							{/if}
						</div>
					{/each}
				</div>
			{:else}
				<fieldset class="flex gap-6">
					<label class="flex items-center gap-2 text-sm cursor-pointer">
						<input
							type="radio"
							name="mode"
							checked={mode === 'same'}
							onchange={() => handleModeChange('same')}
							class="accent-primary"
						/>
						{$t('applications.migration.sameDest')}
					</label>
					<label class="flex items-center gap-2 text-sm cursor-pointer">
						<input
							type="radio"
							name="mode"
							checked={mode === 'individual'}
							onchange={() => handleModeChange('individual')}
							class="accent-primary"
						/>
						{$t('applications.migration.individualDest')}
					</label>
				</fieldset>

				{#if mode === 'same'}
					<div class="flex items-center gap-3">
						<span class="text-sm text-muted-foreground shrink-0">{$t('applications.migration.allVolumes')}</span>
						<select
							class="border-input bg-background ring-offset-background focus-visible:ring-ring flex h-9 w-full rounded-md border px-3 py-1 text-sm shadow-xs focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
							value={sameDest}
							onchange={(e) => handleSameDestChange((e.target as HTMLSelectElement).value)}
							disabled={migrating}
						>
							<option value="" disabled>{$t('applications.migration.selectDrive')}</option>
							{#each drives as drive (drive.mountpoint)}
								<option value={drive.mountpoint}>
									{drive.device} — {drive.mountpoint} ({drive.free_gb} Go libre)
								</option>
							{/each}
						</select>
					</div>
				{/if}

				<div class="rounded-lg border">
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head class="w-10">
									<Checkbox.Root checked={allChecked()} onclick={toggleAll} disabled={migrating} />
								</Table.Head>
								<Table.Head>Volume</Table.Head>
								<Table.Head class="hidden sm:table-cell">Source</Table.Head>
								<Table.Head>{$t('applications.migration.select')}</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each volumes as vol, i (vol.name)}
								<Table.Row class={!vol.IsMigratable ? 'opacity-50' : ''}>
									<Table.Cell>
										<Checkbox.Root bind:checked={volumes[i].checked} disabled={migrating || !vol.IsMigratable} />
									</Table.Cell>
									<Table.Cell class="font-medium">
										{vol.name}
										{#if !vol.IsMigratable}
											<span class="ml-1 text-xs text-muted-foreground">({$t('applications.inMemory')})</span>
										{/if}
									</Table.Cell>
									<Table.Cell class="hidden sm:table-cell font-mono text-xs text-muted-foreground max-w-48 truncate">{vol.source}</Table.Cell>
									<Table.Cell>
										<select
											class="border-input bg-background ring-offset-background focus-visible:ring-ring flex h-8 w-full rounded-md border px-2 py-1 text-xs shadow-xs focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
											value={vol.destination}
											onchange={(e) => handleRowDestChange(i, (e.target as HTMLSelectElement).value)}
											disabled={migrating || !vol.IsMigratable}
										>
											<option value="" disabled>{$t('applications.migration.select')}</option>
											{#each drives as drive (drive.mountpoint)}
												<option value={drive.mountpoint}>{drive.device} — {drive.mountpoint}</option>
											{/each}
										</select>
									</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</div>
			{/if}

			{#if resultMessage}
				<p
					class="text-sm"
					class:text-green-600={resultMessage === $t('applications.migration.success')}
					class:text-destructive={resultMessage !== $t('applications.migration.success')}
				>
					{resultMessage}
				</p>
			{/if}
		</div>

		<Dialog.Footer class="flex gap-2">
			<Button variant="outline" onclick={() => handleModalClose()} disabled={migrating && !currentJob?.status}>
				{$t('applications.migration.cancel')}
			</Button>
			{#if !currentJobId}
				<Button onclick={handleMigrate} disabled={migrating || !volumes.some(v => v.IsMigratable && v.checked)}>
					{#if migrating}<LoaderCircle class="size-4 animate-spin" />{/if}
					{$t('applications.migration.migrate')}
				</Button>
			{:else if currentJob?.status === 'completed' || currentJob?.status === 'failed'}
				<Button onclick={() => handleModalClose()}>{$t('applications.migration.close')}</Button>
			{:else}
				<Button disabled>
					<LoaderCircle class="size-4 animate-spin" />
					{$t('applications.migration.running')}…
				</Button>
			{/if}
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<style>
</style>
