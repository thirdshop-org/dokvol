<script lang="ts">
	import { onMount } from 'svelte';
	import { getDrives, checkDriveHealth, initDrive } from '$lib/api';
	import { t } from '$lib/i18n';
	import type { DriveInfo } from '$lib/types';
	import { Button } from '$lib/components/ui/button/index.js';
	import { LoaderCircle } from '@lucide/svelte';

	let drives = $state<DriveInfo[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let healthMap = $state<Record<string, boolean | null>>({});
	let checkingHealth = $state(false);
	let initializing = $state<Record<string, boolean>>({});

	onMount(async () => {
		try {
			drives = await getDrives();
		} catch (e) {
			error = e instanceof Error ? e.message : $t('drives.error.unknown');
		} finally {
			loading = false;
		}
		for (const d of drives) {
			checkOneHealth(d);
		}
	});

	async function checkAllHealth() {
		checkingHealth = true;
		await Promise.all(drives.map(d => checkOneHealth(d)));
		checkingHealth = false;
	}

	async function checkOneHealth(drive: DriveInfo) {
		try {
			const res = await checkDriveHealth(drive.mountpoint);
			healthMap[drive.mountpoint] = res.healthy;
		} catch {
			healthMap[drive.mountpoint] = false;
		}
	}

	async function handleInit(drive: DriveInfo) {
		initializing[drive.mountpoint] = true;
		try {
			await initDrive(drive.mountpoint);
			await checkOneHealth(drive);
		} catch {
			healthMap[drive.mountpoint] = false;
		} finally {
			initializing[drive.mountpoint] = false;
		}
	}
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight">{$t('drives.title')}</h1>
			<p class="text-muted-foreground">{$t('drives.description')}</p>
		</div>
		<Button onclick={checkAllHealth} disabled={checkingHealth}>
			{#if checkingHealth}
				<LoaderCircle class="size-4 animate-spin" />
			{/if}
			{$t('drives.checkHealth')}
		</Button>
	</div>

	{#if loading}
		<p class="text-muted-foreground">{$t('drives.loading')}</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else}
		<div class="rounded-lg border overflow-x-auto">
			<table class="w-full text-sm">
				<thead class="border-b bg-muted/50 text-muted-foreground">
					<tr>
						<th class="px-4 py-3 text-left font-medium">{$t('drives.table.device')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('drives.table.mountpoint')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('drives.table.filesystem')}</th>
						<th class="px-4 py-3 text-right font-medium">{$t('drives.table.total')}</th>
						<th class="px-4 py-3 text-right font-medium">{$t('drives.table.free')}</th>
						<th class="px-4 py-3 text-right font-medium">{$t('drives.table.usage')}</th>
						<th class="px-4 py-3 text-center font-medium">{$t('drives.table.dokvolStatus')}</th>
						<th class="px-4 py-3 text-center font-medium">{$t('drives.table.action')}</th>
						<th class="px-4 py-3 text-center font-medium">{$t('stats.evolution')}</th>
					</tr>
				</thead>
				<tbody>
					{#each drives as drive (drive.device)}
						{@const healthy = healthMap[drive.mountpoint]}
						<tr class="border-b last:border-0 hover:bg-muted/30">
							<td class="px-4 py-3 font-mono text-xs">{drive.device}</td>
							<td class="px-4 py-3 font-mono text-xs">{drive.mountpoint}</td>
							<td class="px-4 py-3">{drive.fstype}</td>
							<td class="px-4 py-3 text-right">{drive.total_gb} Go</td>
							<td class="px-4 py-3 text-right">{drive.free_gb} Go</td>
							<td class="px-4 py-3 text-right">
								<div class="flex items-center justify-end gap-2">
									<div class="h-2 w-20 overflow-hidden rounded-full bg-muted">
										<div
											class="h-full rounded-full transition-all"
											class:bg-destructive={drive.used_pct > 90}
											class:bg-yellow-500={drive.used_pct > 75 && drive.used_pct <= 90}
											class:bg-green-500={drive.used_pct <= 75}
											style="width: {drive.used_pct}%"
										></div>
									</div>
									<span class="w-12 text-right">{drive.used_pct.toFixed(1)}%</span>
								</div>
							</td>
							<td class="px-4 py-3 text-center">
								{#if healthy === null}
									<LoaderCircle class="mx-auto size-4 animate-spin text-muted-foreground" />
								{:else if healthy}
									<span class="badge badge-ok">{$t('drives.initialized')}</span>
								{:else}
									<span class="badge badge-missing">{$t('drives.notInitialized')}</span>
								{/if}
							</td>
							<td class="px-4 py-3 text-center">
								{#if initializing[drive.mountpoint]}
									<Button size="sm" disabled>
										<LoaderCircle class="size-3 animate-spin" />
									</Button>
								{:else if healthy === false}
									<Button size="sm" onclick={() => handleInit(drive)}>
										{$t('drives.initialize')}
									</Button>
								{:else if healthy === true}
									<span class="text-xs text-muted-foreground">{$t('drives.check')}</span>
								{/if}
							</td>
							<td class="px-4 py-3 text-center">
								<a href="/stats?tab=drives&mountpoint={encodeURIComponent(drive.mountpoint)}" class="text-xs text-muted-foreground hover:text-foreground underline">{$t('stats.evolution')}</a>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<style>
	.badge {
		display: inline-flex;
		align-items: center;
		border-radius: 9999px;
		padding: 0.125rem 0.5rem;
		font-size: 0.75rem;
		font-weight: 500;
	}
	.badge-ok {
		background-color: #dcfce7;
		color: #166534;
	}
	.badge-missing {
		background-color: #fef2f2;
		color: #991b1b;
	}
	:global(.dark) .badge-ok {
		background-color: #14532d;
		color: #bbf7d0;
	}
	:global(.dark) .badge-missing {
		background-color: #450a0a;
		color: #fecaca;
	}
</style>
