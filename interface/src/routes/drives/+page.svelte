<script lang="ts">
	import { onMount } from 'svelte';
	import { getDrives } from '$lib/api';
	import type { DriveInfo } from '$lib/types';

	let drives = $state<DriveInfo[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(async () => {
		try {
			drives = await getDrives();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Erreur inconnue';
		} finally {
			loading = false;
		}
	});
</script>

<div class="space-y-6">
	<div>
		<h1 class="text-2xl font-bold tracking-tight">Disques</h1>
		<p class="text-muted-foreground">Périphériques de stockage disponibles pour Docker.</p>
	</div>

	{#if loading}
		<p class="text-muted-foreground">Chargement...</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else}
		<div class="rounded-lg border">
			<table class="w-full text-sm">
				<thead class="border-b bg-muted/50 text-muted-foreground">
					<tr>
						<th class="px-4 py-3 text-left font-medium">Périphérique</th>
						<th class="px-4 py-3 text-left font-medium">Point de montage</th>
						<th class="px-4 py-3 text-left font-medium">Système de fichiers</th>
						<th class="px-4 py-3 text-right font-medium">Taille totale</th>
						<th class="px-4 py-3 text-right font-medium">Libre</th>
						<th class="px-4 py-3 text-right font-medium">Utilisation</th>
					</tr>
				</thead>
				<tbody>
					{#each drives as drive (drive.device)}
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
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
