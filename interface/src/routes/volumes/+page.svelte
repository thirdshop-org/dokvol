<script lang="ts">
	import { onMount } from 'svelte';
	import { getVolumes } from '$lib/api';
	import { t } from '$lib/i18n';
	import type { VolumeDetail } from '$lib/types';

	let volumes = $state<VolumeDetail[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(async () => {
		try {
			volumes = await getVolumes();
		} catch (e) {
			error = e instanceof Error ? e.message : $t('volumes.error.unknown');
		} finally {
			loading = false;
		}
	});
</script>

<div class="space-y-6">
	<div>
		<h1 class="text-2xl font-bold tracking-tight">{$t('volumes.title')}</h1>
		<p class="text-muted-foreground">{$t('volumes.description')}</p>
	</div>

	{#if loading}
		<p class="text-muted-foreground">{$t('volumes.loading')}</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else}
		<div class="rounded-lg border">
			<table class="w-full text-sm">
				<thead class="border-b bg-muted/50 text-muted-foreground">
					<tr>
						<th class="px-4 py-3 text-left font-medium">{$t('volumes.table.container')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('volumes.table.type')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('volumes.table.source')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('volumes.table.destination')}</th>
						<th class="px-4 py-3 text-center font-medium">{$t('stats.evolution')}</th>
					</tr>
				</thead>
				<tbody>
					{#each volumes as vol, i (i)}
						<tr class="border-b last:border-0 hover:bg-muted/30">
							<td class="px-4 py-3 font-medium">{vol.ContainerName.replace(/^\//, '')}</td>
							<td class="px-4 py-3">
								<span class="badge {vol.Type}">{vol.Type}</span>
							</td>
							<td class="px-4 py-3 font-mono text-xs">{vol.Source}</td>
							<td class="px-4 py-3 font-mono text-xs">{vol.Destination}</td>
							<td class="px-4 py-3 text-center">
								<a href="/stats/volumes?name={encodeURIComponent(vol.Name || vol.Source)}" class="text-xs text-muted-foreground hover:text-foreground underline">{$t('stats.evolution')}</a>
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
