<script lang="ts">
	import { onMount } from 'svelte';
	import { getDrives, getVolumes, getApplications } from '$lib/api';
	import { HardDrive, Layers, Box } from '@lucide/svelte';

	let driveCount = $state(0);
	let volumeCount = $state(0);
	let appCount = $state(0);
	let ready = $state(false);

	onMount(async () => {
		try {
			const [drives, volumes, apps] = await Promise.all([
				getDrives(),
				getVolumes(),
				getApplications(),
			]);
			driveCount = drives.length;
			volumeCount = volumes.length;
			appCount = apps.length;
		} catch {
			// silencieux
		} finally {
			ready = true;
		}
	});
</script>

<div class="space-y-6">
	<div>
		<h1 class="text-2xl font-bold tracking-tight">Tableau de bord</h1>
		<p class="text-muted-foreground">Aperçu de votre infrastructure Docker.</p>
	</div>

	{#if !ready}
		<p class="text-muted-foreground">Chargement...</p>
	{:else}
		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			<a
				href="/drives"
				class="card-link border-l-green-500"
			>
				<div>
					<p class="text-sm font-medium text-muted-foreground">Disques</p>
					<p class="text-3xl font-bold">{driveCount}</p>
				</div>
				<HardDrive class="size-8 text-muted-foreground" />
			</a>
			<a
				href="/volumes"
				class="card-link border-l-blue-500"
			>
				<div>
					<p class="text-sm font-medium text-muted-foreground">Volumes</p>
					<p class="text-3xl font-bold">{volumeCount}</p>
				</div>
				<Layers class="size-8 text-muted-foreground" />
			</a>
			<a
				href="/applications"
				class="card-link border-l-amber-500"
			>
				<div>
					<p class="text-sm font-medium text-muted-foreground">Applications</p>
					<p class="text-3xl font-bold">{appCount}</p>
				</div>
				<Box class="size-8 text-muted-foreground" />
			</a>
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
