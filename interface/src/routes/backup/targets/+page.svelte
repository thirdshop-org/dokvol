<script lang="ts">
	import { onMount } from 'svelte';
	import { getBackupTargets, deleteBackupTarget, testBackupTarget } from '$lib/api';
	import type { BackupTarget } from '$lib/types';
	import { Button } from '$lib/components/ui/button/index.js';
	import { LoaderCircle, Plus, Trash2, Plug, ArrowRight } from '@lucide/svelte';
	import { errorMessage } from '$lib/utils/errors';
	import { t } from '$lib/i18n';
	import ConfirmDialog from '$lib/components/confirm-dialog.svelte';

	let targets = $state<BackupTarget[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let testing = $state<Record<string, boolean>>({});
	let deleting = $state<Record<string, boolean>>({});
	let testResult = $state<Record<string, { success: boolean; message: string } | null>>({});

	let deleteTarget = $state<BackupTarget | null>(null);
	let deleteError = $state<string | null>(null);

	onMount(async () => {
		try {
			targets = await getBackupTargets();
		} catch (e) {
			error = errorMessage(e);
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
			testResult[id] = { success: false, message: errorMessage(e) };
		} finally {
			testing[id] = false;
		}
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		const id = deleteTarget.id;
		deleting[id] = true;
		deleteError = null;
		try {
			await deleteBackupTarget(id);
			targets = targets.filter(t => t.id !== id);
			deleteTarget = null;
		} catch (e) {
			deleteError = errorMessage(e);
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
			<h1 class="text-2xl font-bold tracking-tight">{$t('backup.targets.title')}</h1>
			<p class="text-muted-foreground">{$t('backup.targets.description')}</p>
		</div>
		<a href="/backup/targets/new">
			<Button>
				<Plus class="size-4" />
				{$t('backup.newTarget')}
			</Button>
		</a>
	</div>

	{#if loading}
		<p class="text-muted-foreground">{$t('backup.loading')}</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else if targets.length === 0}
		<div class="rounded-lg border border-dashed p-12 text-center">
			<p class="text-muted-foreground">{$t('backup.targets.empty')}</p>
			<a href="/backup/targets/new" class="mt-2 inline-block">
				<Button variant="outline">
					<Plus class="size-4" />
					{$t('backup.targets.createFirst')}
				</Button>
			</a>
		</div>
	{:else}
		<div class="rounded-lg border overflow-x-auto">
			<table class="w-full text-sm">
				<thead class="border-b bg-muted/50 text-muted-foreground">
					<tr>
						<th class="px-4 py-3 text-left font-medium">{$t('backup.targets.table.name')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('backup.targets.table.provider')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('backup.targets.table.created')}</th>
						<th class="px-4 py-3 text-right font-medium">{$t('backup.targets.table.actions')}</th>
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
										{$t('backup.targets.test')}
									</Button>
									<Button size="sm" variant="destructive" onclick={() => (deleteTarget = target)} disabled={deleting[target.id]}>
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

<ConfirmDialog
	open={deleteTarget !== null}
	onOpenChange={(v) => { if (!v) { deleteTarget = null; deleteError = null; } }}
	title={$t('backup.targets.confirmDelete')}
	confirmLabel={$t('backup.delete')}
	cancelLabel={$t('backup.cancel')}
	loading={deleteTarget ? deleting[deleteTarget.id] : false}
	error={deleteError}
	onconfirm={handleDelete}
/>
