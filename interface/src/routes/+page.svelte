<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { getDrives, getVolumes, getApplications, getSystemHealth } from '$lib/api';
	import { errorMessage } from '$lib/utils/errors';
	import { HardDrive, Layers, Box, Stethoscope, Check, X } from '@lucide/svelte';
	import type { SystemHealthResponse } from '$lib/types';

	let driveCount = $state(0);
	let volumeCount = $state(0);
	let appCount = $state(0);
	let ready = $state(false);
	let health = $state<SystemHealthResponse | null>(null);
	let healthError = $state<string | null>(null);

	onMount(async () => {
		try {
			const [drives, volumes, apps, h] = await Promise.all([
				getDrives(),
				getVolumes(),
				getApplications(),
				getSystemHealth().catch((e: unknown) => {
					healthError = errorMessage(e);
					return null;
				}),
			]);
			driveCount = drives.length;
			volumeCount = volumes.length;
			appCount = apps.length;
			health = h;
		} catch {
			// silencieux
		} finally {
			ready = true;
		}
	});
</script>

<div class="space-y-6">
	<div>
		<h1 class="text-2xl font-bold tracking-tight">{$t('dashboard.title')}</h1>
		<p class="text-muted-foreground">{$t('dashboard.description')}</p>
	</div>

	{#if !ready}
		<p class="text-muted-foreground">{$t('dashboard.loading')}</p>
	{:else}
		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
			<a
				href="/drives"
				class="card-link border-l-green-500"
			>
				<div>
					<p class="text-sm font-medium text-muted-foreground">{$t('dashboard.cards.drives')}</p>
					<p class="text-3xl font-bold">{driveCount}</p>
				</div>
				<HardDrive class="size-8 text-muted-foreground" />
			</a>
			<a
				href="/volumes"
				class="card-link border-l-blue-500"
			>
				<div>
					<p class="text-sm font-medium text-muted-foreground">{$t('dashboard.cards.volumes')}</p>
					<p class="text-3xl font-bold">{volumeCount}</p>
				</div>
				<Layers class="size-8 text-muted-foreground" />
			</a>
			<a
				href="/applications"
				class="card-link border-l-amber-500"
			>
				<div>
					<p class="text-sm font-medium text-muted-foreground">{$t('dashboard.cards.applications')}</p>
					<p class="text-3xl font-bold">{appCount}</p>
				</div>
				<Box class="size-8 text-muted-foreground" />
			</a>
			<div class="card-link border-l-gray-500">
				<div class="min-w-0">
					<p class="text-sm font-medium text-muted-foreground">{$t('dashboard.health.title')}</p>
					{#if healthError}
						<p class="text-sm text-destructive truncate" title={healthError}>{healthError}</p>
					{:else if health?.healthy}
						<div class="flex items-center gap-1 text-green-600">
							<Check class="size-4" />
							<span class="text-sm font-medium">{$t('dashboard.health.ok')}</span>
						</div>
					{:else if health && !health.healthy}
						<div class="flex items-center gap-1 text-destructive">
							<X class="size-4" />
							<span class="text-sm font-medium">{$t('dashboard.health.fail')}</span>
						</div>
					{:else}
						<p class="text-sm text-muted-foreground">—</p>
					{/if}
				</div>
				<Stethoscope class="size-8 text-muted-foreground shrink-0" />
			</div>
		</div>
	{/if}
</div>

<style>
	.card-link {
		display: flex;
		align-items: center;
		justify-content: space-between;
		border-radius: 0.5rem;
		border-width: 1px;
		border-left-width: 4px;
		background-color: hsl(var(--card));
		padding: 1rem;
		box-shadow: 0 1px 2px 0 rgb(0 0 0 / 0.05);
		transition: background-color 0.15s;
		text-decoration: none;
	}
	.card-link:hover {
		background-color: hsl(var(--accent));
	}
</style>
