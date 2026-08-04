<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { getBackupJobs } from '$lib/api';
	import type { BackupJob } from '$lib/types';
	import { LoaderCircle } from '@lucide/svelte';
	import { statusBadgeClass } from '$lib/utils/status';
	import { formatBytes } from '$lib/utils/format';
	import { errorMessage } from '$lib/utils/errors';
	import { t } from '$lib/i18n';

	let jobs = $state<BackupJob[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	async function load() {
		try {
			const res = await getBackupJobs({ limit: 50 });
			jobs = res.jobs;
			total = res.total;
		} catch (e) {
			error = errorMessage(e);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		load();
		pollTimer = setInterval(load, 5000);
	});

	onDestroy(() => {
		if (pollTimer) clearInterval(pollTimer);
	});

	function hasActive(): boolean {
		return jobs.some(j => j.status === 'running' || j.status === 'pending');
	}

	function formatDuration(ms: number): string {
		if (!ms) return '—';
		const s = Math.floor(ms / 1000);
		const m = Math.floor(s / 60);
		const h = Math.floor(m / 60);
		if (h > 0) return `${h}h ${m % 60}m`;
		if (m > 0) return `${m}m ${s % 60}s`;
		return `${s}s`;
	}


</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight">{$t('backup.jobs.title')}</h1>
			<p class="text-muted-foreground">
				{$t('backup.jobs.totalCount', { n: total })}
				{#if hasActive()}
					<span class="ml-2 inline-flex items-center gap-1 text-primary">
						<LoaderCircle class="size-3 animate-spin" />
						{$t('backup.jobs.activeRunning')}
					</span>
				{/if}
			</p>
		</div>
	</div>

	{#if loading}
		<p class="text-muted-foreground">{$t('backup.loading')}</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else if jobs.length === 0}
		<div class="rounded-lg border border-dashed p-12 text-center">
			<p class="text-muted-foreground">{$t('backup.noJobs')}</p>
		</div>
	{:else}
		<div class="rounded-lg border overflow-x-auto">
			<table class="w-full text-sm">
				<thead class="border-b bg-muted/50 text-muted-foreground">
					<tr>
						<th class="px-4 py-3 text-left font-medium">{$t('backup.jobs.table.app')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('backup.jobs.table.status')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('backup.jobs.table.size')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('backup.jobs.table.started')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('backup.jobs.table.duration')}</th>
					</tr>
				</thead>
				<tbody>
					{#each jobs as job (job.id)}
						<tr class="border-b last:border-0 hover:bg-muted/30 cursor-pointer" onclick={() => window.location.href = `/backup/jobs/${job.id}`}>
							<td class="px-4 py-3 font-medium">{job.app_name}</td>
							<td class="px-4 py-3">
								<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium {statusBadgeClass(job.status)}">
									{#if job.status === 'running' || job.status === 'pending'}
										<LoaderCircle class="size-3 animate-spin" />
									{/if}
									{job.status}
								</span>
							</td>
							<td class="px-4 py-3 text-muted-foreground">{formatBytes(job.total_bytes)}</td>
							<td class="px-4 py-3 text-muted-foreground">{job.started_at ? new Date(job.started_at).toLocaleString() : '—'}</td>
							<td class="px-4 py-3 text-muted-foreground font-mono text-xs">{formatDuration(job.duration_ms)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
