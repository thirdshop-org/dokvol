<script lang="ts">
	import { onMount } from 'svelte';
	import { getVolumes, getDrives, migrateVolume, deleteVolumes, ApiError } from '$lib/api';
	import { t } from '$lib/i18n';
	import type { VolumeDetail, DriveInfo } from '$lib/types';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Checkbox from '$lib/components/ui/checkbox/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import FileExplorer from '$lib/components/file-explorer.svelte';
	import { LoaderCircle, Trash2, ArrowUpFromLine, Search, Undo2 } from '@lucide/svelte';

	type VolumeRow = VolumeDetail & { checked: boolean };

	let rows = $state<VolumeRow[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let drives = $state<DriveInfo[]>([]);

	let moveDialogOpen = $state(false);
	let deleteDialogOpen = $state(false);
	let migrating = $state(false);
	let selectedDrive = $state('');
	let bulkMessage = $state<string | null>(null);
	let bulkSuccess = $state(false);

	let deleting = $state(false);

	let browseOpen = $state(false);
	let browseTarget = $state<{ container: string; path: string; volumeName: string } | null>(null);

	onMount(async () => {
		try {
			const vols = await getVolumes();
			rows = vols.map(v => ({ ...v, checked: false }));
			drives = await getDrives();
		} catch (e) {
			error = e instanceof Error ? e.message : $t('volumes.error.unknown');
		} finally {
			loading = false;
		}
	});

	$effect(() => {
		if (!moveDialogOpen && !deleteDialogOpen) {
			bulkMessage = null;
		}
	});

	function allChecked() {
		return rows.length > 0 && rows.every(r => r.checked);
	}

	function toggleAll() {
		const newVal = !allChecked();
		for (const r of rows) r.checked = newVal;
	}

	function selectedRows() {
		return rows.filter(r => r.checked);
	}

	function selectedCount() {
		return rows.filter(r => r.checked).length;
	}

	function errorMessage(e: unknown): string {
		if (e instanceof ApiError) {
			const key = `error.${e.errorCode}`;
			const msg = $t(key);
			return msg !== key ? msg : e.message;
		}
		return e instanceof Error ? e.message : $t('volumes.error.unknown');
	}

	async function handleMove() {
		const selected = selectedRows();
		if (selected.length === 0) {
			bulkMessage = $t('volumes.bulk.noSelection');
			bulkSuccess = false;
			return;
		}
		if (!selectedDrive) {
			bulkMessage = $t('volumes.bulk.selectDrive');
			bulkSuccess = false;
			return;
		}

		migrating = true;
		bulkMessage = null;

		try {
			const grouped = new Map<string, VolumeDetail[]>();
			for (const vol of selected) {
				const app = vol.ContainerName;
				if (!grouped.has(app)) grouped.set(app, []);
				grouped.get(app)!.push(vol);
			}

			for (const [app, vols] of grouped) {
				const namedVols = vols.filter(v => v.Name).map(v => ({
					name: v.Name,
					destination_mountpoint: selectedDrive,
				}));

				if (namedVols.length > 0) {
					await migrateVolume({
						application: app,
						volumes: namedVols,
					});
				} else {
					await migrateVolume({
						application: app,
						destination_mountpoint: selectedDrive,
					});
				}
			}

			bulkMessage = $t('volumes.bulk.migrateSuccess');
			bulkSuccess = true;
		} catch (e) {
			bulkMessage = errorMessage(e);
			bulkSuccess = false;
		} finally {
			migrating = false;
		}
	}

	async function handleDelete() {
		const selected = selectedRows();
		if (selected.length === 0) return;

		deleting = true;
		bulkMessage = null;

		try {
			const deletable = selected.filter(v => v.Type === 'volume' && v.Name);
			const res = await deleteVolumes({
				volumes: deletable.map(v => ({
					name: v.Name,
					source: v.Source,
					type: v.Type,
				})),
			});

			if (res.success) {
				const remaining = rows.filter(r => !r.checked || r.Type !== 'volume' || !r.Name);
				rows = remaining;
				bulkMessage = $t('volumes.bulk.deleteSuccess');
				bulkSuccess = true;
			} else {
				bulkMessage = res.errors?.join(', ') || $t('volumes.bulk.deleteError');
				bulkSuccess = false;
			}
		} catch (e) {
			bulkMessage = errorMessage(e);
			bulkSuccess = false;
		} finally {
			deleting = false;
		}
	}
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight">{$t('volumes.title')}</h1>
			<p class="text-muted-foreground">{$t('volumes.description')}</p>
		</div>
		<a href="/volumes/trash">
			<Button variant="outline" size="sm">
				<Undo2 class="size-3.5" />
				{$t('trash.title')}
			</Button>
		</a>
	</div>

	{#if loading}
		<p class="text-muted-foreground">{$t('volumes.loading')}</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else}
		{#if selectedCount() > 0}
			<div class="flex items-center justify-between rounded-lg border bg-muted/30 px-4 py-2">
				<span class="text-sm text-muted-foreground">
					{$t('volumes.bulk.selected', { n: selectedCount() })}
				</span>
				<div class="flex gap-2">
					<Button size="sm" variant="outline" onclick={() => (moveDialogOpen = true)}>
						<ArrowUpFromLine class="size-3.5" />
						{$t('volumes.bulk.move')}
					</Button>
					<Button size="sm" variant="destructive" onclick={() => (deleteDialogOpen = true)}>
						<Trash2 class="size-3.5" />
						{$t('volumes.bulk.delete')}
					</Button>
				</div>
			</div>
		{/if}

		<div class="rounded-lg border">
			<table class="w-full text-sm">
				<thead class="border-b bg-muted/50 text-muted-foreground">
					<tr>
						<th class="w-10 px-4 py-3 text-left">
							<Checkbox.Root checked={allChecked()} onclick={toggleAll} />
						</th>
						<th class="px-4 py-3 text-left font-medium">{$t('volumes.table.container')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('volumes.table.type')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('volumes.table.source')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('volumes.table.destination')}</th>
						<th class="px-4 py-3 text-center font-medium">{$t('stats.evolution')}</th>
						<th class="px-4 py-3 text-center font-medium">{$t('fileExplorer.browse')}</th>
					</tr>
				</thead>
				<tbody>
					{#each rows as row, i (i)}
						<tr class="border-b last:border-0 hover:bg-muted/30" class:opacity-50={row.Type !== 'volume'}>
							<td class="px-4 py-3">
								<Checkbox.Root bind:checked={rows[i].checked} />
							</td>
							<td class="px-4 py-3 font-medium">{row.ContainerName.replace(/^\//, '')}</td>
							<td class="px-4 py-3">
								<span class="badge {row.Type}">{row.Type}</span>
							</td>
							<td class="px-4 py-3 font-mono text-xs">{row.Source}</td>
							<td class="px-4 py-3 font-mono text-xs">{row.Destination}</td>
							<td class="px-4 py-3 text-center">
								<a href="/stats/volumes?name={encodeURIComponent(row.Name || row.Source)}"
								   class="text-xs text-muted-foreground hover:text-foreground underline">
								   {$t('stats.evolution')}
								</a>
							</td>
							<td class="px-4 py-3 text-center">
								<button
									onclick={() => { browseTarget = { container: row.ContainerName, path: row.Destination, volumeName: row.Name || row.Destination }; browseOpen = true; }}
									class="inline-flex items-center justify-center size-7 rounded-md hover:bg-accent hover:text-accent-foreground transition-colors text-muted-foreground"
									aria-label={$t('fileExplorer.browse')}
								>
									<Search class="size-3.5" />
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<Dialog.Root bind:open={moveDialogOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{$t('volumes.bulk.move')}</Dialog.Title>
			<Dialog.Description>
				{$t('volumes.bulk.selected', { n: selectedCount() })} — {$t('volumes.bulk.destination')}
			</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-4">
			<select
				class="border-input bg-background ring-offset-background focus-visible:ring-ring flex h-9 w-full rounded-md border px-3 py-1 text-sm shadow-xs focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
				bind:value={selectedDrive}
				disabled={migrating}
			>
				<option value="">{$t('volumes.bulk.selectDrive')}</option>
				{#each drives as drive (drive.mountpoint)}
					<option value={drive.mountpoint}>
						{drive.device} — {drive.mountpoint} ({drive.free_gb} Go libre)
					</option>
				{/each}
			</select>

			{#if bulkMessage}
				<p class="text-sm" class:text-green-600={bulkSuccess} class:text-destructive={!bulkSuccess}>
					{bulkMessage}
				</p>
			{/if}
		</div>
		<Dialog.Footer class="flex gap-2">
			<Button variant="outline" onclick={() => (moveDialogOpen = false)} disabled={migrating}>
				{$t('volumes.bulk.cancel')}
			</Button>
			<Button onclick={handleMove} disabled={migrating || !selectedDrive}>
				{#if migrating}<LoaderCircle class="size-4 animate-spin" />{/if}
				{$t('volumes.bulk.migrate')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<Dialog.Root bind:open={deleteDialogOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{$t('volumes.bulk.confirmDelete')}</Dialog.Title>
			<Dialog.Description>
				{$t('volumes.bulk.confirmDeleteDesc')}
			</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-4">
			{#if bulkMessage}
				<p class="text-sm" class:text-green-600={bulkSuccess} class:text-destructive={!bulkSuccess}>
					{bulkMessage}
				</p>
			{/if}
		</div>
		<Dialog.Footer class="flex gap-2">
			<Button variant="outline" onclick={() => (deleteDialogOpen = false)} disabled={deleting}>
				{$t('volumes.bulk.cancel')}
			</Button>
			<Button variant="destructive" onclick={handleDelete} disabled={deleting}>
				{#if deleting}<LoaderCircle class="size-4 animate-spin" />{/if}
				{$t('volumes.bulk.delete')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<Sheet.Root bind:open={browseOpen}>
	<Sheet.Content side="right" class="sm:max-w-2xl p-0">
		{#if browseTarget}
			<FileExplorer container={browseTarget.container} initialPath={browseTarget.path} volumeName={browseTarget.volumeName} />
		{/if}
	</Sheet.Content>
</Sheet.Root>

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
