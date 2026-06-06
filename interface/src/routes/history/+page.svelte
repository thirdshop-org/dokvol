<script lang="ts">
	import { onMount } from 'svelte';
	import { getHistory, getHistoryJob, rescanHistory, getHistoryAppNames, ApiError } from '$lib/api';
	import { t } from '$lib/i18n';
	import type { MigrationLogEntry, HistoryJobDetail } from '$lib/types';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { LoaderCircle, RotateCw, Search, ChevronDown } from '@lucide/svelte';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Table from '$lib/components/ui/table/index.js';
	import { Input } from '$lib/components/ui/input/index.js';

	let entries = $state<MigrationLogEntry[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let rescanning = $state(false);
	let rescanMessage = $state<string | null>(null);

	let appNames = $state<string[]>([]);
	let filterApp = $state('');
	let filterAppOpen = $state(false);
	let filterAppFocused = $state(false);
	let filterStatus = $state('');
	let offset = $state(0);
	const limit = 50;

	let selectedJobId = $state<string | null>(null);
	let jobDetail = $state<HistoryJobDetail | null>(null);
	let detailLoading = $state(false);
	let detailOpen = $state(false);

	function formatBytes(bytes: number): string {
		if (!Number.isFinite(bytes) || bytes <= 0) return '—';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		const i = Math.floor(Math.log(bytes) / Math.log(1024));
		return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
	}

	function formatDuration(ms: number): string {
		if (ms <= 0) return '—';
		if (ms < 1000) return ms + ' ms';
		if (ms < 60000) return (ms / 1000).toFixed(1) + ' s';
		const min = Math.floor(ms / 60000);
		const sec = ((ms % 60000) / 1000).toFixed(0);
		return min + ' min ' + sec + ' s';
	}

	function formatDate(s: string | undefined): string {
		if (!s) return '—';
		const d = new Date(s);
		return d.toLocaleDateString() + ' ' + d.toLocaleTimeString();
	}

	function statusBadgeClass(status: string): string {
		switch (status) {
			case 'completed': return 'bg-green-100 text-green-800 hover:bg-green-100 dark:bg-green-900 dark:text-green-100 dark:hover:bg-green-900';
			case 'failed': return 'bg-red-100 text-red-800 hover:bg-red-100 dark:bg-red-900 dark:text-red-100 dark:hover:bg-red-900';
			default: return 'bg-yellow-100 text-yellow-800 hover:bg-yellow-100 dark:bg-yellow-900 dark:text-yellow-100 dark:hover:bg-yellow-900';
		}
	}

	async function load() {
		loading = true;
		error = null;
		try {
			const params: Record<string, string | number> = { limit, offset };
			if (filterApp) params.app = filterApp;
			if (filterStatus) params.status = filterStatus;
			const resp = await getHistory(params as any);
			entries = resp.entries;
			total = resp.total;
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Failed to load history';
		} finally {
			loading = false;
		}
	}

	async function handleRescan() {
		rescanning = true;
		rescanMessage = null;
		try {
			await rescanHistory();
			rescanMessage = $t('history.rescanSuccess');
			await load();
		} catch {
			rescanMessage = 'Failed to rescan';
		} finally {
			rescanning = false;
		}
	}

	async function openDetail(jobId: string) {
		selectedJobId = jobId;
		detailLoading = true;
		detailOpen = true;
		try {
			jobDetail = await getHistoryJob(jobId);
		} catch {
			jobDetail = null;
		} finally {
			detailLoading = false;
		}
	}

	function applyFilter() {
		offset = 0;
		load();
	}

	onMount(async () => {
		try { appNames = await getHistoryAppNames(); } catch { /* ignore */ }
		load();
	});
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight">{$t('history.title')}</h1>
			<p class="text-muted-foreground">{$t('history.description')}</p>
		</div>
		<Button variant="outline" onclick={handleRescan} disabled={rescanning}>
			{#if rescanning}
				<LoaderCircle class="size-4 animate-spin" />
			{:else}
				<RotateCw class="size-4" />
			{/if}
			{$t('history.rescan')}
		</Button>
	</div>

	{#if rescanMessage}
		<p class="text-sm text-green-600">{rescanMessage}</p>
	{/if}

	<div class="flex flex-wrap gap-3 items-end">
		<div class="relative" role="combobox" aria-expanded={filterAppOpen}>
			<Search class="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
			<Input
				bind:value={filterApp}
				placeholder={$t('history.filters.byApp')}
				class="pl-9 w-56"
				oninput={() => { filterAppOpen = true; applyFilter(); }}
				onfocus={() => { filterAppOpen = true; filterAppFocused = true; }}
				onblur={() => setTimeout(() => { filterAppOpen = false; filterAppFocused = false; }, 150)}
			/>
			{#if filterAppOpen && appNames.length > 0}
				<div class="absolute top-full left-0 z-50 mt-1 w-full rounded-md border bg-background shadow-lg max-h-48 overflow-y-auto">
					{#each appNames.filter(n => !filterApp || n.toLowerCase().includes(filterApp.toLowerCase())) as name}
						<button
							class="w-full px-3 py-1.5 text-left text-sm hover:bg-muted"
							onmousedown={() => { filterApp = name; filterAppOpen = false; applyFilter(); }}
						>{name}</button>
					{/each}
				</div>
			{/if}
		</div>
		<select
			class="border-input bg-background ring-offset-background focus-visible:ring-ring flex h-9 rounded-md border px-3 py-1 text-sm shadow-xs focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none"
			bind:value={filterStatus}
			onchange={applyFilter}
		>
			<option value="">{$t('history.filters.all')}</option>
			<option value="completed">{$t('history.filters.completed')}</option>
			<option value="failed">{$t('history.filters.failed')}</option>
		</select>
	</div>

	{#if loading}
		<p class="text-muted-foreground">{$t('history.loading')}</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else if entries.length === 0}
		<p class="text-muted-foreground">{$t('history.empty')}</p>
	{:else}
		<div class="rounded-lg border">
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>{$t('history.columns.app')}</Table.Head>
						<Table.Head>{$t('history.columns.volume')}</Table.Head>
						<Table.Head class="hidden md:table-cell">{$t('history.columns.source')}</Table.Head>
						<Table.Head class="hidden md:table-cell">{$t('history.columns.destination')}</Table.Head>
						<Table.Head>{$t('history.columns.status')}</Table.Head>
						<Table.Head>{$t('history.columns.size')}</Table.Head>
						<Table.Head class="hidden lg:table-cell">{$t('history.columns.duration')}</Table.Head>
						<Table.Head>{$t('history.columns.date')}</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each entries as entry (entry.id)}
						<Table.Row class="cursor-pointer hover:bg-muted/30" onclick={() => openDetail(entry.job_id)}>
							<Table.Cell class="font-medium">{entry.app_name}</Table.Cell>
							<Table.Cell class="font-mono text-xs">{entry.volume_name}</Table.Cell>
							<Table.Cell class="hidden md:table-cell font-mono text-xs text-muted-foreground max-w-40 truncate" title={entry.source_path}>
								{entry.source_drive ? entry.source_drive : entry.source_path}
							</Table.Cell>
							<Table.Cell class="hidden md:table-cell font-mono text-xs text-muted-foreground max-w-40 truncate" title={entry.dest_path}>
								{entry.dest_drive}
							</Table.Cell>
							<Table.Cell>
								<Badge class={statusBadgeClass(entry.status)}>{entry.status}</Badge>
							</Table.Cell>
							<Table.Cell class="font-mono text-xs text-muted-foreground">{formatBytes(entry.total_bytes)}</Table.Cell>
							<Table.Cell class="hidden lg:table-cell font-mono text-xs text-muted-foreground">{formatDuration(entry.duration_ms)}</Table.Cell>
							<Table.Cell class="font-mono text-xs text-muted-foreground">{formatDate(entry.created_at)}</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>

		{#if total > limit}
			<div class="flex justify-center gap-2">
				<Button variant="outline" disabled={offset === 0} onclick={() => { offset = Math.max(0, offset - limit); load(); }}>
					{$t('navigation.previous') || 'Previous'}
				</Button>
				<span class="flex items-center text-sm text-muted-foreground">
					{offset + 1}–{Math.min(offset + limit, total)} / {total}
				</span>
				<Button variant="outline" disabled={offset + limit >= total} onclick={() => { offset += limit; load(); }}>
					{$t('navigation.next') || 'Next'}
				</Button>
			</div>
		{/if}
	{/if}
</div>

<Dialog.Root bind:open={detailOpen}>
	<Dialog.Content class="sm:max-w-2xl">
		<Dialog.Header>
			<Dialog.Title>{$t('history.details.title')}</Dialog.Title>
		</Dialog.Header>
		<div class="space-y-3 max-h-96 overflow-y-auto">
			{#if detailLoading}
				<p class="text-sm text-muted-foreground">{$t('history.loading')}</p>
			{:else if jobDetail}
				<div class="flex items-center gap-2 mb-2">
					<span class="font-semibold">{jobDetail.app_name}</span>
					<Badge class={statusBadgeClass(jobDetail.status)}>{jobDetail.status}</Badge>
					{#if jobDetail.started_at}
						<span class="text-xs text-muted-foreground">{formatDate(jobDetail.started_at)}</span>
					{/if}
				</div>
				{#each jobDetail.volumes as vol}
					<div class="rounded-lg border p-3 text-sm">
						<div class="flex items-center justify-between mb-2">
							<span class="font-medium">{vol.volume_name}</span>
							<Badge class={statusBadgeClass(vol.status)}>{vol.status}</Badge>
						</div>
						<div class="grid grid-cols-2 gap-2 text-xs text-muted-foreground">
							<div>
								<span class="block font-medium text-foreground">Source</span>
								<span class="truncate block" title={vol.source_path}>{vol.source_path}</span>
								{#if vol.source_drive}
									<span class="block">Drive: {vol.source_drive}</span>
								{/if}
							</div>
							<div>
								<span class="block font-medium text-foreground">Destination</span>
								<span class="truncate block" title={vol.dest_path}>{vol.dest_path}</span>
								<span class="block">Drive: {vol.dest_drive}</span>
							</div>
						</div>
						<div class="flex gap-4 mt-2 text-xs text-muted-foreground">
							<span>{formatBytes(vol.total_bytes)}</span>
							<span>{formatDuration(vol.duration_ms)}</span>
							{#if vol.completed_at}
								<span>{formatDate(vol.completed_at)}</span>
							{/if}
						</div>
						{#if vol.error_message}
							<p class="mt-1 text-xs text-destructive">{vol.error_message}</p>
						{/if}
					</div>
				{/each}
			{:else}
				<p class="text-sm text-muted-foreground">{$t('history.details.noInfo')}</p>
			{/if}
		</div>
		<Dialog.Footer>
			<Button onclick={() => (detailOpen = false)}>{$t('navigation.close') || 'Close'}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<style>
</style>
