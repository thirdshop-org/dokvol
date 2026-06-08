<script lang="ts">
	import { onMount } from 'svelte';
	import { getBackupTargets, deleteBackupTarget, testBackupTarget } from '$lib/api';
	import type { BackupTarget } from '$lib/types';
	import { Button } from '$lib/components/ui/button/index.js';
	import { LoaderCircle, Plus, Trash2, Plug, ArrowRight } from '@lucide/svelte';

	let targets = $state<BackupTarget[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let testing = $state<Record<string, boolean>>({});
	let deleting = $state<Record<string, boolean>>({});
	let testResult = $state<Record<string, { success: boolean; message: string } | null>>({});

	onMount(async () => {
		try {
			targets = await getBackupTargets();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load targets';
		} finally {
			loading = false;
		}
	});

	async function handleTest(id: string) {
		testing[id] = true;
		try {
			const res = await testBackupTarget(id);
			testResult[id] = res;
		} catch (e) {
			testResult[id] = { success: false, message: e instanceof Error ? e.message : 'Test failed' };
		} finally {
			testing[id] = false;
		}
	}

	async function handleDelete(id: string) {
		if (!confirm('Delete this backup target?')) return;
		deleting[id] = true;
		try {
			await deleteBackupTarget(id);
			targets = targets.filter(t => t.id !== id);
		} catch (e) {
			alert(e instanceof Error ? e.message : 'Delete failed');
		} finally {
			deleting[id] = false;
		}
	}

	function providerIcon(provider: string): string {
		switch (provider) {
			case 's3': return 'S3';
			case 'sftp': return 'SFTP';
			case 'local': return 'Local';
			default: return provider;
		}
	}
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight">Backup Targets</h1>
			<p class="text-muted-foreground">Configure where backups are stored</p>
		</div>
		<a href="/backup/targets/new">
			<Button>
				<Plus class="size-4" />
				New Target
			</Button>
		</a>
	</div>

	{#if loading}
		<p class="text-muted-foreground">Loading...</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else if targets.length === 0}
		<div class="rounded-lg border border-dashed p-12 text-center">
			<p class="text-muted-foreground">No backup targets configured</p>
			<a href="/backup/targets/new" class="mt-2 inline-block">
				<Button variant="outline">
					<Plus class="size-4" />
					Create your first target
				</Button>
			</a>
		</div>
	{:else}
		<div class="rounded-lg border">
			<table class="w-full text-sm">
				<thead class="border-b bg-muted/50 text-muted-foreground">
					<tr>
						<th class="px-4 py-3 text-left font-medium">Name</th>
						<th class="px-4 py-3 text-left font-medium">Provider</th>
						<th class="px-4 py-3 text-left font-medium">Created</th>
						<th class="px-4 py-3 text-right font-medium">Actions</th>
					</tr>
				</thead>
				<tbody>
					{#each targets as target (target.id)}
						<tr class="border-b last:border-0 hover:bg-muted/30">
							<td class="px-4 py-3">
								<a href="/backup/targets/{target.id}" class="flex items-center gap-2 font-medium hover:underline">
									{target.name}
									<ArrowRight class="size-3 text-muted-foreground" />
								</a>
							</td>
							<td class="px-4 py-3">
								<span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800 dark:bg-blue-900 dark:text-blue-100">
									{providerIcon(target.provider)}
								</span>
							</td>
							<td class="px-4 py-3 text-muted-foreground">
								{new Date(target.created_at).toLocaleDateString()}
							</td>
							<td class="px-4 py-3">
								<div class="flex items-center justify-end gap-2">
									<Button size="sm" variant="outline" onclick={() => handleTest(target.id)} disabled={testing[target.id]}>
										{#if testing[target.id]}
											<LoaderCircle class="size-3 animate-spin" />
										{:else}
											<Plug class="size-3" />
										{/if}
										Test
									</Button>
									<Button size="sm" variant="destructive" onclick={() => handleDelete(target.id)} disabled={deleting[target.id]}>
										{#if deleting[target.id]}
											<LoaderCircle class="size-3 animate-spin" />
										{:else}
											<Trash2 class="size-3" />
										{/if}
									</Button>
								</div>
								{#if testResult[target.id]}
									<p class="mt-1 text-right text-xs" class:text-green-600={testResult[target.id]!.success} class:text-destructive={!testResult[target.id]!.success}>
										{testResult[target.id]!.message}
									</p>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
