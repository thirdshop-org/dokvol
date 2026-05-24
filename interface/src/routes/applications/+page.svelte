<script lang="ts">
	import { onMount } from 'svelte';
	import { getApplications } from '$lib/api';
	import type { ApplicationVolumes } from '$lib/types';

	let apps = $state<ApplicationVolumes[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(async () => {
		try {
			apps = await getApplications();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Erreur inconnue';
		} finally {
			loading = false;
		}
	});
</script>

<div class="space-y-6">
	<div>
		<h1 class="text-2xl font-bold tracking-tight">Applications</h1>
		<p class="text-muted-foreground">Conteneurs Docker et leurs volumes montés.</p>
	</div>

	{#if loading}
		<p class="text-muted-foreground">Chargement...</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else}
		{#each apps as app (app.ContainerName)}
			<div class="rounded-lg border">
				<div class="border-b bg-muted/30 px-4 py-3 font-semibold">
					{app.ContainerName.replace(/^\//, '')}
					<span class="ml-2 text-xs font-normal text-muted-foreground">
						({app.Volumes.length} volume{app.Volumes.length > 1 ? 's' : ''})
					</span>
				</div>
				<table class="w-full text-sm">
					<thead class="border-b text-muted-foreground">
						<tr>
							<th class="px-4 py-2 text-left font-medium">Type</th>
							<th class="px-4 py-2 text-left font-medium">Source</th>
							<th class="px-4 py-2 text-left font-medium">Destination</th>
						</tr>
					</thead>
					<tbody>
						{#each app.Volumes as vol, i (i)}
							<tr class="border-b last:border-0 hover:bg-muted/30">
								<td class="px-4 py-2">
									<span class="badge {vol.Type}">{vol.Type}</span>
								</td>
								<td class="px-4 py-2 font-mono text-xs">{vol.Source}</td>
								<td class="px-4 py-2 font-mono text-xs">{vol.Destination}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/each}
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
	.badge.volume {
		background-color: #dbeafe;
		color: #1e40af;
	}
	.badge.bind {
		background-color: #fef3c7;
		color: #92400e;
	}
	:global(.dark) .badge.volume {
		background-color: #1e3a5f;
		color: #bfdbfe;
	}
	:global(.dark) .badge.bind {
		background-color: #5c3d0e;
		color: #fde68a;
	}
</style>
