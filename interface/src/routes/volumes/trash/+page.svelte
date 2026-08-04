<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { getTrash, restoreTrashEntry, purgeTrashEntry, ApiError } from '$lib/api';
	import type { TrashEntry } from '$lib/types';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import * as Table from '$lib/components/ui/table/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { LoaderCircle, Undo2, Trash2, ArrowLeft } from '@lucide/svelte';
	import { statusBadgeClass } from '$lib/utils/status';

	let entries = $state<TrashEntry[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let restoreTarget = $state<TrashEntry | null>(null);
	let purgeTarget = $state<TrashEntry | null>(null);
	let acting = $state(false);
	let actionError = $state<string | null>(null);

	function errorMessage(e: unknown): string {
		if (e instanceof ApiError) {
			const key = `error.${e.errorCode}`;
			const msg = $t(key);
			return msg !== key ? msg : e.message;
		}
		return e instanceof Error ? e.message : $t('error.default');
	}

	async function load() {
		loading = true;
		error = null;
		try {
			entries = await getTrash();
		} catch (e) {
			error = errorMessage(e);
		} finally {
			loading = false;
		}
	}

	onMount(load);

	function volumeLabel(entry: TrashEntry): string {
		return entry.volume_name || entry.source_path;
	}

	async function handleRestore() {
		if (!restoreTarget) return;
		acting = true;
		actionError = null;
		try {
			await restoreTrashEntry(restoreTarget.id);
			restoreTarget = null;
			await load();
		} catch (e) {
			actionError = errorMessage(e);
		} finally {
			acting = false;
		}
	}

	async function handlePurge() {
		if (!purgeTarget) return;
		acting = true;
		actionError = null;
		try {
			await purgeTrashEntry(purgeTarget.id);
			purgeTarget = null;
			await load();
		} catch (e) {
			actionError = errorMessage(e);
		} finally {
			acting = false;
		}
	}
</script>

<div class="space-y-6">
	<div>
		<a href="/volumes" class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
			<ArrowLeft class="size-4" />
			{$t('volumes.title')}
		</a>
		<h1 class="mt-2 text-2xl font-bold tracking-tight">{$t('trash.title')}</h1>
		<p class="text-muted-foreground">{$t('trash.description')}</p>
	</div>

	{#if loading}
		<p class="text-muted-foreground">{$t('trash.loading')}</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else if entries.length === 0}
		<div class="rounded-lg border border-dashed p-12 text-center">
			<p class="text-muted-foreground">{$t('trash.empty')}</p>
		</div>
	{:else}
		<div class="rounded-lg border">
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>{$t('trash.table.app')}</Table.Head>
						<Table.Head>{$t('trash.table.volume')}</Table.Head>
						<Table.Head>{$t('trash.table.step')}</Table.Head>
						<Table.Head class="hidden md:table-cell">{$t('trash.table.backupPath')}</Table.Head>
						<Table.Head class="text-right">{$t('trash.table.actions')}</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each entries as entry (entry.id)}
						<Table.Row>
							<Table.Cell class="font-medium">{entry.app_name.replace(/^\//, '')}</Table.Cell>
							<Table.Cell class="font-mono text-xs">{volumeLabel(entry)}</Table.Cell>
							<Table.Cell>
								<Badge class={statusBadgeClass(entry.step)}>{entry.step}</Badge>
							</Table.Cell>
							<Table.Cell class="hidden md:table-cell font-mono text-xs text-muted-foreground max-w-64 truncate" title={entry.backup_path}>
								{entry.backup_path}
							</Table.Cell>
							<Table.Cell class="text-right">
								<div class="flex justify-end gap-2">
									<Button size="sm" variant="outline" onclick={() => (restoreTarget = entry)}>
										<Undo2 class="size-3.5" />
										{$t('trash.restore')}
									</Button>
									<Button size="sm" variant="destructive" onclick={() => (purgeTarget = entry)}>
										<Trash2 class="size-3.5" />
										{$t('trash.purge')}
									</Button>
								</div>
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>
	{/if}
</div>

<Dialog.Root open={restoreTarget !== null} onOpenChange={(v) => { if (!v) { restoreTarget = null; actionError = null; } }}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{$t('trash.confirmRestoreTitle')}</Dialog.Title>
			<Dialog.Description>
				{$t('trash.confirmRestoreDesc', { volume: restoreTarget ? volumeLabel(restoreTarget) : '' })}
			</Dialog.Description>
		</Dialog.Header>
		{#if actionError}
			<p class="text-sm text-destructive">{actionError}</p>
		{/if}
		<Dialog.Footer class="flex gap-2">
			<Button variant="outline" onclick={() => (restoreTarget = null)} disabled={acting}>
				{$t('trash.cancel')}
			</Button>
			<Button onclick={handleRestore} disabled={acting}>
				{#if acting}<LoaderCircle class="size-4 animate-spin" />{/if}
				{$t('trash.restore')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<Dialog.Root open={purgeTarget !== null} onOpenChange={(v) => { if (!v) { purgeTarget = null; actionError = null; } }}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{$t('trash.confirmPurgeTitle')}</Dialog.Title>
			<Dialog.Description>
				{$t('trash.confirmPurgeDesc', { volume: purgeTarget ? volumeLabel(purgeTarget) : '' })}
			</Dialog.Description>
		</Dialog.Header>
		{#if actionError}
			<p class="text-sm text-destructive">{actionError}</p>
		{/if}
		<Dialog.Footer class="flex gap-2">
			<Button variant="outline" onclick={() => (purgeTarget = null)} disabled={acting}>
				{$t('trash.cancel')}
			</Button>
			<Button variant="destructive" onclick={handlePurge} disabled={acting}>
				{#if acting}<LoaderCircle class="size-4 animate-spin" />{/if}
				{$t('trash.purge')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
