<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { getApplications, getMigrationStatus, getActiveMigrations, getHistory, getHistoryJob, getStatsVolume, stopApplication, startApplication, restartApplication } from '$lib/api';
	import { t } from '$lib/i18n';
	import { errorMessage } from '$lib/utils/errors';
	import type { ApplicationVolumes, VolumeDetail, MigrationJob, MigrationLogEntry, HistoryJobDetail, StatsVolume } from '$lib/types';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { ArrowLeft, Clock, LoaderCircle, Square, Play, RotateCcw, Search } from '@lucide/svelte';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import FileExplorer from '$lib/components/file-explorer.svelte';
	import { formatBytes } from '$lib/utils/format';

	let loading = $state(true);
	let error = $state<string | null>(null);
	let app = $state<ApplicationVolumes | null>(null);
	let activeJob = $state<MigrationJob | null>(null);
	let historyEntries = $state<MigrationLogEntry[]>([]);
	let historyDetail = $state<HistoryJobDetail | null>(null);
	let historyDetailLoading = $state(false);
	let historyDetailOpen = $state(false);
	let volumeStats = $state<Record<string, StatsVolume | null>>({});
	let now = $state(Date.now());

	let pollTimer: ReturnType<typeof setInterval> | null = null;
	let elapsedTimer: ReturnType<typeof setInterval> | null = null;

	let browseOpen = $state(false);
	let browseTarget = $state<{ container: string; path: string; volumeName: string } | null>(null);
	let actionLoading = $state<string | null>(null);
	let confirmAction = $state<'stop' | 'restart' | null>(null);
	let confirmOpen = $state(false);

	const appName = $derived($page.params.name);
	const fullAppName = $derived('/' + appName);

	function formatDuration(ms: number): string {
		if (ms <= 0) return '—';
		const totalSec = Math.floor(ms / 1000);
		const h = Math.floor(totalSec / 3600);
		const m = Math.floor((totalSec % 3600) / 60);
		const s = totalSec % 60;
		if (h > 0) return `${h}h ${m}m ${s}s`;
		if (m > 0) return `${m}m ${s}s`;
		return `${s}s`;
	}

	function formatDate(s: string | undefined): string {
		if (!s) return '—';
		return new Date(s).toLocaleString();
	}

	function statusLabel(state: string): string {
		try { return $t(`container.status.${state}`); }
		catch { return state; }
	}

	function statusClass(state: string): string {
		switch (state) {
			case 'running': return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-100';
			case 'exited': return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-100';
			case 'paused': return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-100';
			case 'restarting': return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-100';
			case 'removing': case 'dead': return 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-100';
			default: return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-100';
		}
	}

	function stepBadge(step: string): string {
		switch (step) {
			case 'completed': return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-100';
			case 'failed': return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-100';
			case 'pending': return 'bg-muted text-muted-foreground';
			default: return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-100';
		}
	}

	onMount(async () => {
		try {
			const apps = await getApplications();
			const found = apps.find(a => a.ContainerName.replace(/^\//, '') === appName);
			if (!found) {
				error = 'Application not found';
				loading = false;
				return;
			}
			app = found;

			const volNames = new Set<string>();
			for (const vol of app.Volumes) {
				const key = vol.Name || vol.Source.split('/').filter(Boolean).pop() || '';
				if (key) volNames.add(key);
			}
			const volResults = await Promise.allSettled(
				[...volNames].map(name =>
					getStatsVolume(name).then(stats => ({ name, stats: stats.at(-1) ?? null }))
				)
			);
			const volMap: Record<string, StatsVolume | null> = {};
			for (const r of volResults) {
				if (r.status === 'fulfilled') volMap[r.value.name] = r.value.stats;
			}
			volumeStats = volMap;

			const allJobs = await getActiveMigrations();
			const job = allJobs.find(j => j.app_name === fullAppName);
			if (job) {
				activeJob = job;
				startPoll(job.id);
			}

			const hist = await getHistory({ app: fullAppName, limit: 50 });
			historyEntries = hist.entries;
		} catch (e) {
			error = errorMessage(e);
		} finally {
			loading = false;
		}
	});

	function startPoll(jobId: string) {
		pollTimer = setInterval(async () => {
			try {
				const job = await getMigrationStatus(jobId);
				activeJob = job;
			} catch { /* ignore */ }
		}, 2000);

		elapsedTimer = setInterval(() => { now = Date.now(); }, 1000);
	}

	function stopPoll() {
		if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
		if (elapsedTimer) { clearInterval(elapsedTimer); elapsedTimer = null; }
	}

	onDestroy(() => stopPoll());

	async function openHistoryDetail(jobId: string) {
		historyDetailLoading = true;
		historyDetailOpen = true;
		try { historyDetail = await getHistoryJob(jobId); }
		catch { historyDetail = null; }
		finally { historyDetailLoading = false; }
	}

	function handleAction(action: 'stop' | 'start' | 'restart') {
		if (action === 'stop' || action === 'restart') {
			confirmAction = action;
			confirmOpen = true;
			return;
		}
		executeAction(action);
	}

	async function executeAction(action: 'stop' | 'start' | 'restart') {
		actionLoading = action;
		try {
			if (action === 'stop') await stopApplication(fullAppName);
			else if (action === 'start') await startApplication(fullAppName);
			else await restartApplication(fullAppName);
			const apps = await getApplications();
			const found = apps.find(a => a.ContainerName.replace(/^\//, '') === appName);
			if (found) app = found;
		} catch {
			// handled via loading state
		} finally {
			actionLoading = null;
			confirmAction = null;
			confirmOpen = false;
		}
	}
</script>

<div class="space-y-6">
	{#if loading}
		<p class="text-muted-foreground">{$t('applications.loading')}</p>
	{:else if error}
		<div class="space-y-2">
			<p class="text-destructive">{error}</p>
			<Button variant="outline" href="/applications"><ArrowLeft class="size-3.5" /> {$t('applications.title')}</Button>
		</div>
	{:else if app}
		<!-- Header -->
		<div class="flex items-start justify-between gap-3">
			<div class="flex items-center gap-3">
				<Button variant="outline" size="sm" href="/applications"><ArrowLeft class="size-3.5" /></Button>
				<div>
					<h1 class="text-2xl font-bold tracking-tight">{appName}</h1>
					<div class="flex items-center gap-2 text-sm text-muted-foreground">
						<Badge class={statusClass(app.Status)}>{statusLabel(app.Status)}</Badge>
						<span>({$t('applications.volumeCount', { n: app.Volumes.length })})</span>
					</div>
				</div>
			</div>
			<div class="flex gap-2">
				{#if app.Status === 'running'}
					<Button size="sm" variant="outline" onclick={() => handleAction('stop')} disabled={actionLoading !== null}>
						{#if actionLoading === 'stop'}
							<LoaderCircle class="size-3.5 animate-spin" />
						{:else}
							<Square class="size-3.5" />
						{/if}
						{$t('applications.stop')}
					</Button>
					<Button size="sm" variant="outline" onclick={() => handleAction('restart')} disabled={actionLoading !== null}>
						{#if actionLoading === 'restart'}
							<LoaderCircle class="size-3.5 animate-spin" />
						{:else}
							<RotateCcw class="size-3.5" />
						{/if}
						{$t('applications.restart')}
					</Button>
				{:else}
					<Button size="sm" variant="outline" onclick={() => handleAction('start')} disabled={actionLoading !== null}>
						{#if actionLoading === 'start'}
							<LoaderCircle class="size-3.5 animate-spin" />
						{:else}
							<Play class="size-3.5" />
						{/if}
						{$t('applications.start')}
					</Button>
				{/if}
			</div>
		</div>

		<!-- Active Migration -->
		{#if activeJob}
			{@const jobStart = new Date(activeJob.created_at).getTime()}
			{@const elapsed = activeJob.status === 'running' ? now - jobStart : 0}
			<div class="rounded-lg border border-primary/30 bg-primary/5">
				<div class="flex items-center justify-between border-b border-primary/20 px-4 py-3">
					<h2 class="flex items-center gap-2 font-semibold text-sm">
						<LoaderCircle class="size-4 animate-spin text-primary" />
						{$t('applications.migration.running')}
					</h2>
					<div class="flex items-center gap-2 text-sm">
						<Clock class="size-3.5 text-muted-foreground" />
						<span class="font-mono">{formatDuration(elapsed)}</span>
						<Badge class="bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-100">{activeJob.status}</Badge>
					</div>
				</div>
				<div class="divide-y">
					{#each activeJob.volumes as vol}
						<div class="px-4 py-3">
							<div class="flex items-center justify-between">
								<div class="flex items-center gap-2 text-sm">
									{#if vol.step === 'completed'}
										<span class="text-green-600 dark:text-green-400">✓</span>
									{:else if vol.step === 'failed'}
										<span class="text-red-600 dark:text-red-400">✗</span>
									{:else if vol.step === 'pending'}
										<span class="text-muted-foreground">○</span>
									{:else}
										<LoaderCircle class="size-4 animate-spin text-primary" />
									{/if}
									<span class="font-medium">{vol.volume_name}</span>
									<Badge class={stepBadge(vol.step)}>{$t('step.' + vol.step)}</Badge>
								</div>
								{#if vol.total_bytes > 0}
									<span class="text-xs text-muted-foreground">{formatBytes(vol.transferred_bytes)} / {formatBytes(vol.total_bytes)}</span>
								{/if}
							</div>
							{#if vol.step === 'syncing' && vol.total_bytes > 0}
								<div class="mt-2 h-2 w-full overflow-hidden rounded-full bg-muted">
									<div class="h-full rounded-full bg-primary transition-all" style="width: {Math.min(100, (vol.transferred_bytes / vol.total_bytes) * 100)}%"></div>
								</div>
							{/if}
							{#if vol.created_at}
								{@const volElapsed = vol.step === 'completed'
									? new Date(vol.updated_at).getTime() - new Date(vol.created_at).getTime()
									: now - new Date(vol.created_at).getTime()}
								<p class="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
									<Clock class="size-3" /> {formatDuration(volElapsed)}
								</p>
							{/if}
						</div>
					{/each}
				</div>
				<div class="flex flex-wrap items-center gap-1 border-t border-primary/20 px-4 py-2 text-xs text-muted-foreground">
					<Badge class="bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-100 text-[10px]">{$t('step.stopping')}</Badge>
					{#each activeJob.volumes as vol, i}
						<span class="mx-0.5">→</span>
						<Badge class={vol.step === 'completed' ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-100' : vol.step === 'syncing' ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-100' : 'bg-muted text-muted-foreground' + ' text-[10px]'}>{vol.volume_name}</Badge>
					{/each}
					<span class="mx-0.5">→</span>
					<Badge class="bg-muted text-muted-foreground text-[10px]">{$t('step.starting')}</Badge>
				</div>
			</div>
		{/if}

		<!-- Volumes -->
		<div class="rounded-lg border">
			<div class="border-b bg-muted/30 px-4 py-2 font-semibold text-sm">{$t('volumes.title')}</div>
			<div class="overflow-x-auto">
			<table class="w-full text-sm">
				<thead class="border-b text-muted-foreground">
					<tr>
						<th class="px-4 py-2 text-left font-medium">{$t('volumes.table.type')}</th>
						<th class="px-4 py-2 text-left font-medium">{$t('volumes.table.size')}</th>
						<th class="px-4 py-2 text-left font-medium">{$t('volumes.table.source')}</th>
						<th class="px-4 py-2 text-left font-medium">{$t('volumes.table.drive')}</th>
						<th class="px-4 py-2 text-left font-medium">{$t('volumes.table.destination')}</th>
						<th class="px-4 py-2 text-center font-medium">{$t('fileExplorer.browse')}</th>
					</tr>
				</thead>
				<tbody>
					{#each app.Volumes as vol, i (i)}
						{@const volKey = vol.Name || vol.Source.split('/').filter(Boolean).pop() || ''}
						{@const vs = volumeStats[volKey]}
						<tr class="border-b last:border-0 hover:bg-muted/30">
							<td class="px-4 py-2">
								{#if !vol.IsMigratable}
									<Badge class="bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-100">{vol.Type} — {$t('applications.inMemory')}</Badge>
								{:else}
									<Badge class={vol.Type === 'volume' ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-100' : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-100'}>{vol.Type}</Badge>
								{/if}
							</td>
							<td class="px-4 py-2 font-mono text-xs text-muted-foreground">{vs?.total_bytes != null ? formatBytes(vs.total_bytes) : '—'}</td>
							<td class="px-4 py-2 font-mono text-xs">
								{vol.Source}
								{#if vol.MigratedDriveMountpoint}
									<Badge class="bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-100 ml-1 text-[10px]">→ {vol.MigratedDriveMountpoint}</Badge>
								{/if}
							</td>
							<td class="px-4 py-2 font-mono text-xs">
								{#if vol.SystemDrive}
									<Tooltip.Root>
										<Tooltip.Trigger>
											<Badge class="bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-100 text-[10px] cursor-default">{vol.SystemDrive.device}</Badge>
										</Tooltip.Trigger>
										<Tooltip.Content side="top" align="center">{vol.SystemDrive.mountpoint}</Tooltip.Content>
									</Tooltip.Root>
								{:else}
									<span class="text-muted-foreground">—</span>
								{/if}
							</td>
							<td class="px-4 py-2 font-mono text-xs">{vol.Destination}</td>
							<td class="px-4 py-2 text-center">
								<button
									onclick={() => { browseTarget = { container: vol.ContainerName, path: vol.Destination, volumeName: vol.Name || vol.Destination }; browseOpen = true; }}
									class="inline-flex items-center justify-center size-7 rounded-md hover:bg-accent hover:text-accent-foreground transition-colors text-muted-foreground"
									aria-label={$t('fileExplorer.browse')}
								>
									<Search class="size-3.5" />
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			</div>
		</div>

		<!-- History -->
		<div class="rounded-lg border">
			<div class="border-b bg-muted/30 px-4 py-2 font-semibold text-sm">{$t('history.title')}</div>
			{#if historyEntries.length === 0}
				<p class="px-4 py-6 text-sm text-muted-foreground text-center">{$t('history.empty')}</p>
			{:else}
				<div class="overflow-x-auto">
				<table class="w-full text-sm">
					<thead class="border-b text-muted-foreground">
						<tr>
							<th class="px-4 py-2 text-left font-medium">{$t('history.columns.date')}</th>
							<th class="px-4 py-2 text-left font-medium">{$t('history.columns.status')}</th>
							<th class="px-4 py-2 text-left font-medium">{$t('history.columns.volume')}</th>
							<th class="px-4 py-2 text-left font-medium">{$t('history.columns.destination')}</th>
							<th class="px-4 py-2 text-left font-medium">{$t('history.columns.size')}</th>
							<th class="px-4 py-2 text-left font-medium">{$t('history.columns.duration')}</th>
						</tr>
					</thead>
					<tbody>
						{#each historyEntries as entry (entry.id)}
							<tr
								class="border-b last:border-0 hover:bg-muted/30 cursor-pointer"
								role="button"
								tabindex={0}
								onclick={() => openHistoryDetail(entry.job_id)}
								onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); openHistoryDetail(entry.job_id); } }}
							>
								<td class="px-4 py-2 text-xs text-muted-foreground">{formatDate(entry.created_at)}</td>
								<td class="px-4 py-2">
									<Badge class={entry.status === 'completed' ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-100' : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-100'}>
										{entry.status === 'completed' ? $t('history.filters.completed') : $t('history.filters.failed')}
									</Badge>
								</td>
								<td class="px-4 py-2 font-mono text-xs">{entry.volume_name}</td>
								<td class="px-4 py-2 font-mono text-xs">{entry.dest_drive}</td>
								<td class="px-4 py-2 font-mono text-xs">{formatBytes(entry.total_bytes)}</td>
								<td class="px-4 py-2 font-mono text-xs">{formatDuration(entry.duration_ms)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
				</div>
			{/if}
		</div>
	{/if}
</div>

<!-- History Detail Dialog -->
<Dialog.Root bind:open={historyDetailOpen}>
	<Dialog.Content class="sm:max-w-2xl">
		<Dialog.Header>
			<Dialog.Title>{$t('history.details.title')}</Dialog.Title>
		</Dialog.Header>
		{#if historyDetailLoading}
			<p class="text-sm text-muted-foreground">{$t('history.loading')}</p>
		{:else if historyDetail}
			<div class="space-y-3 text-sm">
				<div class="flex items-center gap-4">
					<p><span class="text-muted-foreground">{$t('history.details.jobId')}</span> <span class="font-mono text-xs">{historyDetail.job_id}</span></p>
					<Badge class={historyDetail.status === 'completed' ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-100' : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-100'}>
						{historyDetail.status === 'completed' ? $t('history.filters.completed') : $t('history.filters.failed')}
					</Badge>
				</div>
				<div class="overflow-x-auto">
				<table class="w-full text-sm">
					<thead class="border-b text-muted-foreground">
						<tr>
							<th class="px-3 py-2 text-left font-medium">{$t('history.columns.volume')}</th>
							<th class="px-3 py-2 text-left font-medium">{$t('history.columns.source')}</th>
							<th class="px-3 py-2 text-left font-medium">{$t('history.columns.destination')}</th>
							<th class="px-3 py-2 text-left font-medium">{$t('history.columns.size')}</th>
							<th class="px-3 py-2 text-left font-medium">{$t('history.columns.duration')}</th>
							<th class="px-3 py-2 text-left font-medium">{$t('history.columns.status')}</th>
						</tr>
					</thead>
					<tbody>
						{#each historyDetail.volumes as vol}
							<tr class="border-b last:border-0">
								<td class="px-3 py-2 font-mono text-xs">{vol.volume_name}</td>
								<td class="px-3 py-2 font-mono text-xs max-w-40 truncate" title={vol.source_path}>{vol.source_path}</td>
								<td class="px-3 py-2 font-mono text-xs">{vol.dest_drive}</td>
								<td class="px-3 py-2 font-mono text-xs">{formatBytes(vol.total_bytes)}</td>
								<td class="px-3 py-2 font-mono text-xs">{formatDuration(vol.duration_ms)}</td>
								<td class="px-3 py-2">
									<Badge class={vol.status === 'completed' ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-100 text-[10px]' : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-100 text-[10px]'}>
										{vol.status === 'completed' ? $t('history.filters.completed') : $t('history.filters.failed')}
									</Badge>
								</td>
							</tr>
							{#if vol.error_message}
								<tr class="border-b">
									<td colspan="6" class="px-3 py-1 text-xs text-destructive">{vol.volume_name}: {vol.error_message}</td>
								</tr>
							{/if}
						{/each}
					</tbody>
				</table>
				</div>
			</div>
		{/if}
		<Dialog.Footer>
			<Button onclick={() => (historyDetailOpen = false)}>{$t('applications.migration.close')}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Confirmation Stop / Restart -->
<Dialog.Root bind:open={confirmOpen}>
	<Dialog.Content class="sm:max-w-sm">
		<Dialog.Header>
			<Dialog.Title>
				{confirmAction === 'stop' ? $t('applications.confirm.stopTitle') : $t('applications.confirm.restartTitle')}
			</Dialog.Title>
			<Dialog.Description>
				{confirmAction === 'stop' ? $t('applications.confirm.stopDesc', { name: appName ?? '' }) : $t('applications.confirm.restartDesc', { name: appName ?? '' })}
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer class="flex gap-2">
			<Button variant="outline" onclick={() => { confirmAction = null; confirmOpen = false; }}>
				{$t('applications.confirm.cancel')}
			</Button>
			<Button
				variant={confirmAction === 'stop' ? 'destructive' : 'default'}
				onclick={() => confirmAction && executeAction(confirmAction)}
			>
				{confirmAction === 'stop' ? $t('applications.stop') : $t('applications.restart')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<Sheet.Root bind:open={browseOpen}>
	<Sheet.Content side="right" class="sm:max-w-2xl p-0">
		{#if browseTarget}
			<FileExplorer container={browseTarget.container} initialPath={browseTarget.path} volumeName={browseTarget.volumeName} />
		{/if}
	</Sheet.Content>
</Sheet.Root>
