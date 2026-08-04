<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { getBackupJob } from '$lib/api';
	import type { BackupVolumeProgress } from '$lib/types';
	import { Button } from '$lib/components/ui/button/index.js';
	import { LoaderCircle, ArrowLeft, RotateCcw, Download } from '@lucide/svelte';
	import { formatBytes } from '$lib/utils/format';

	let jobId = $state('');
	let status = $state('');
	let volumes = $state<BackupVolumeProgress[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	async function load() {
		jobId = $page.params.id as string;
		try {
			const res = await getBackupJob(jobId);
			status = res.status;
			volumes = res.volumes;
			if (status !== 'running' && status !== 'pending') {
				if (pollTimer) {
					clearInterval(pollTimer);
					pollTimer = null;
				}
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load job';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		load();
		pollTimer = setInterval(() => {
			load();
		}, 3000);
	});

	onDestroy(() => {
		if (pollTimer) clearInterval(pollTimer);
	});

	function formatDuration(ms: number): string {
		if (!ms) return '—';
		const s = Math.floor(ms / 1000);
		const m = Math.floor(s / 60);
		const h = Math.floor(m / 60);
		if (h > 0) return `${h}h ${m % 60}m ${s % 60}s`;
		if (m > 0) return `${m}m ${s % 60}s`;
		return `${s}s`;
	}

	function progressPct(vol: BackupVolumeProgress): number {
		if (vol.TotalBytes <= 0) return 0;
		return Math.round((vol.TransferredBytes / vol.TotalBytes) * 100);
	}

	function statusClass(s: string): string {
		switch (s) {
			case 'completed': return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-100';
			case 'failed': return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-100';
			case 'running': return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-100';
			case 'pending': return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-100';
			default: return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-100';
		}
	}
</script>

<div class="space-y-6">
	<div>
		<a href="/backup/jobs" class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
			<ArrowLeft class="size-4" />
			Back to jobs
		</a>
	</div>

	{#if loading}
		<p class="text-muted-foreground">Loading...</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else}
		<div class="flex items-center justify-between">
			<div>
				<h1 class="text-2xl font-bold tracking-tight font-mono text-base">{jobId}</h1>
				<p class="text-muted-foreground">
					<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium {statusClass(status)}">
						{#if status === 'running' || status === 'pending'}
							<LoaderCircle class="size-3 animate-spin" />
						{/if}
						{status}
					</span>
				</p>
			</div>
			<div class="flex gap-2">
				{#if status === 'completed'}
					<a href="/backup/restore/{jobId}">
						<Button>
							<Download class="size-4" />
							Restore
						</Button>
					</a>
				{/if}
				{#if status === 'failed'}
					<Button variant="outline" onclick={() => alert('Retry not implemented yet')}>
						<RotateCcw class="size-4" />
						Retry
					</Button>
				{/if}
			</div>
		</div>

		<div class="rounded-lg border">
			<table class="w-full text-sm">
				<thead class="border-b bg-muted/50 text-muted-foreground">
					<tr>
						<th class="px-4 py-3 text-left font-medium">Volume</th>
						<th class="px-4 py-3 text-left font-medium">Source Path</th>
						<th class="px-4 py-3 text-left font-medium">Status</th>
						<th class="px-4 py-3 text-right font-medium">Progress</th>
						<th class="px-4 py-3 text-left font-medium">Error</th>
					</tr>
				</thead>
				<tbody>
					{#each volumes as vol, i (vol.VolumeName || i)}
						<tr class="border-b last:border-0 hover:bg-muted/30">
							<td class="px-4 py-3 font-medium">{vol.VolumeName}</td>
							<td class="px-4 py-3 font-mono text-xs text-muted-foreground">{vol.SourcePath}</td>
							<td class="px-4 py-3">
								<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium {statusClass(vol.Status)}">
									{#if vol.Status === 'running'}
										<LoaderCircle class="size-3 animate-spin" />
									{/if}
									{vol.Status}
								</span>
							</td>
							<td class="px-4 py-3">
								<div class="flex items-center justify-end gap-2">
									<div class="h-2 w-24 overflow-hidden rounded-full bg-muted">
										<div
											class="h-full rounded-full transition-all"
											class:bg-primary={vol.Status !== 'completed' && vol.Status !== 'failed'}
											class:bg-green-500={vol.Status === 'completed'}
											class:bg-destructive={vol.Status === 'failed'}
											style="width: {progressPct(vol)}%"
										></div>
									</div>
									<span class="text-xs text-muted-foreground w-20 text-right">
										{formatBytes(vol.TransferredBytes)} / {formatBytes(vol.TotalBytes)}
									</span>
								</div>
							</td>
							<td class="px-4 py-3 text-xs text-destructive max-w-48 truncate" title={vol.ErrorMessage}>
								{vol.ErrorMessage || '—'}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
