<script lang="ts">
	import { onMount } from 'svelte';
	import { getApplications, getDrives, migrateVolume, ApiError } from '$lib/api';
	import type { ApplicationVolumes, DriveInfo, VolumeDetail } from '$lib/types';
	import { Button } from '$lib/components/ui/button/index.js';
	import { LoaderCircle, ArrowUpFromLine } from '@lucide/svelte';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Table from '$lib/components/ui/table/index.js';
	import * as Checkbox from '$lib/components/ui/checkbox/index.js';

	let apps = $state<ApplicationVolumes[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let modalOpen = $state(false);
	let selectedApp = $state<ApplicationVolumes | null>(null);
	let drives = $state<DriveInfo[]>([]);
	let migrating = $state(false);
	let resultMessage = $state<string | null>(null);

	type VolumeRow = {
		name: string;
		source: string;
		checked: boolean;
		destination: string;
	};

	let volumes = $state<VolumeRow[]>([]);

	let mode = $state<'same' | 'individual'>('same');
	let sameDest = $state<string>('');

	function errorMessage(e: unknown): string {
		if (e instanceof ApiError) {
			switch (e.errorCode) {
				case 'SYSTEM.NOT_FOUND':
					return 'Erreur système';
				case 'DRIVE.NOT_FOUND':
					return 'Disque introuvable';
				case 'DRIVE.HEALTH_CHECK_FAILED':
					return 'Le disque est défaillant';
				case 'APP.NOT_FOUND':
					return 'Application introuvable';
				case 'APP.NO_VOLUMES':
					return 'Aucun volume trouvé pour cette application';
				case 'MIGRATION.AMBIGUOUS_OPTIONS':
					return 'Options de migration ambiguës';
				case 'MIGRATION.SAME_DRIVE':
					return 'Le disque source et le disque de destination sont identiques';
				case 'MIGRATION.NO_DESTINATION':
					return 'Aucun disque de destination sélectionné';
				case 'MIGRATION.VOLUME_NOT_FOUND':
					return 'Volume introuvable';
				case 'MIGRATION.VOLUME_MISMATCH':
					return 'Volume inattendu';
				case 'MIGRATION.INSUFFICIENT_DISK_SPACE':
					return 'Espace disque insuffisant';
				case 'MIGRATION.SYNC_FAILED':
					return 'La synchronisation a échoué';
				case 'MIGRATION.VERIFY_FAILED':
					return 'La vérification a échoué';
				case 'MIGRATION.RELINK_FAILED':
					return 'La reliaison a échoué';
				case 'CONTAINER.STOP_FAILED':
					return "Impossible d'arrêter le conteneur";
				case 'CONTAINER.START_FAILED':
					return 'Impossible de démarrer le conteneur';
				case 'CONTAINER.TIMEOUT':
					return 'Le conteneur a mis trop de temps à s\'arrêter';
				default:
					return e.message;
			}
		}
		return e instanceof Error ? e.message : 'Erreur inconnue';
	}

	onMount(async () => {
		try {
			apps = await getApplications();
		} catch (e) {
			error = errorMessage(e);
		} finally {
			loading = false;
		}
	});

	function openModal(app: ApplicationVolumes) {
		selectedApp = app;
		resultMessage = null;
		mode = 'same';
		sameDest = '';
		volumes = app.Volumes.map((v: VolumeDetail) => ({
			name: v.Name || v.Source.split('/').pop() || 'unknown',
			source: v.Source,
			checked: true,
			destination: '',
		}));
		getDrives().then((d) => (drives = d));
		modalOpen = true;
	}

	function handleModeChange(newMode: 'same' | 'individual') {
		mode = newMode;
		if (newMode === 'same' && sameDest) {
			for (const v of volumes) {
				v.destination = sameDest;
			}
		}
	}

	function handleSameDestChange(mountpoint: string) {
		sameDest = mountpoint;
		for (const v of volumes) {
			v.destination = mountpoint;
		}
	}

	function handleRowDestChange(index: number, mountpoint: string) {
		volumes[index].destination = mountpoint;

		if (mode === 'same') {
			sameDest = mountpoint;
			for (const v of volumes) {
				v.destination = mountpoint;
			}
		} else {
			const checked = volumes.filter((v) => v.checked);
			if (checked.length > 1) {
				for (const v of checked) {
					v.destination = mountpoint;
				}
			}
		}
	}

	function allChecked() {
		return volumes.length > 0 && volumes.every((v) => v.checked);
	}

	function toggleAll() {
		const newVal = !allChecked();
		for (const v of volumes) {
			v.checked = newVal;
		}
	}

	async function handleMigrate() {
		if (!selectedApp) return;
		const appName = selectedApp.ContainerName;

		if (mode === 'same') {
			if (!sameDest) {
				resultMessage = 'Veuillez sélectionner un disque de destination.';
				return;
			}
		} else {
			const selected = volumes.filter((v) => v.checked && v.destination);
			if (selected.length === 0) {
				resultMessage = 'Veuillez sélectionner au moins un volume avec une destination.';
				return;
			}
		}

		migrating = true;
		resultMessage = null;

		try {
			if (mode === 'same') {
				await migrateVolume({
					application: appName,
					destination_mountpoint: sameDest,
				});
			} else {
				const selected = volumes.filter((v) => v.checked && v.destination);
				await migrateVolume({
					application: appName,
					volumes: selected.map((v) => ({
						name: v.name,
						destination_mountpoint: v.destination,
					})),
				});
			}
			resultMessage = 'Migration réussie !';
			apps = await getApplications();
		} catch (e) {
			resultMessage = errorMessage(e);
		} finally {
			migrating = false;
		}
	}
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
				<div class="flex items-center justify-between border-b bg-muted/30 px-4 py-3">
					<h2 class="font-semibold">
						{app.ContainerName.replace(/^\//, '')}
						<span class="ml-2 text-xs font-normal text-muted-foreground">
							({app.Volumes.length} volume{app.Volumes.length > 1 ? 's' : ''})
						</span>
					</h2>
					<Button size="sm" onclick={() => openModal(app)}>
						<ArrowUpFromLine class="size-3.5" />
						Migrer
					</Button>
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

<Dialog.Root bind:open={modalOpen}>
	<Dialog.Content class="sm:max-w-2xl">
		<Dialog.Header>
			<Dialog.Title>
				Migrer les données de « {selectedApp?.ContainerName.replace(/^\//, '')} »
			</Dialog.Title>
			<Dialog.Description>
				Sélectionnez le disque de destination pour chaque volume.
			</Dialog.Description>
		</Dialog.Header>

		<div class="space-y-4">
			<fieldset class="flex gap-6">
				<label class="flex items-center gap-2 text-sm cursor-pointer">
					<input
						type="radio"
						name="mode"
						checked={mode === 'same'}
						onchange={() => handleModeChange('same')}
						class="accent-primary"
					/>
					Même destination pour tous
				</label>
				<label class="flex items-center gap-2 text-sm cursor-pointer">
					<input
						type="radio"
						name="mode"
						checked={mode === 'individual'}
						onchange={() => handleModeChange('individual')}
						class="accent-primary"
					/>
					Destination individuelle
				</label>
			</fieldset>

			{#if mode === 'same'}
				<div class="flex items-center gap-3">
					<span class="text-sm text-muted-foreground shrink-0">Tous les volumes →</span>
					<select
						class="border-input bg-background ring-offset-background focus-visible:ring-ring flex h-9 w-full rounded-md border px-3 py-1 text-sm shadow-xs focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
						value={sameDest}
						onchange={(e) => handleSameDestChange((e.target as HTMLSelectElement).value)}
					>
						<option value="" disabled>Choisir un disque...</option>
						{#each drives as drive (drive.mountpoint)}
							<option value={drive.mountpoint}>
								{drive.device} — {drive.mountpoint} ({drive.free_gb} Go libre)
							</option>
						{/each}
					</select>
				</div>
			{/if}

			<div class="rounded-lg border">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head class="w-10">
								<Checkbox.Root
									checked={allChecked()}
									onclick={toggleAll}
								/>
							</Table.Head>
							<Table.Head>Volume</Table.Head>
							<Table.Head class="hidden sm:table-cell">Source</Table.Head>
							<Table.Head>Destination</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each volumes as vol, i (vol.name)}
							<Table.Row>
								<Table.Cell>
									<Checkbox.Root bind:checked={volumes[i].checked} />
								</Table.Cell>
								<Table.Cell class="font-medium">{vol.name}</Table.Cell>
								<Table.Cell class="hidden sm:table-cell font-mono text-xs text-muted-foreground max-w-48 truncate">
									{vol.source}
								</Table.Cell>
								<Table.Cell>
									<select
										class="border-input bg-background ring-offset-background focus-visible:ring-ring flex h-8 w-full rounded-md border px-2 py-1 text-xs shadow-xs focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
										value={vol.destination}
										onchange={(e) => handleRowDestChange(i, (e.target as HTMLSelectElement).value)}
									>
										<option value="" disabled>Choisir...</option>
										{#each drives as drive (drive.mountpoint)}
											<option value={drive.mountpoint}>
												{drive.device} — {drive.mountpoint}
											</option>
										{/each}
									</select>
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>

			{#if resultMessage}
				<p
					class="text-sm"
					class:text-green-600={resultMessage === 'Migration réussie !'}
					class:text-destructive={resultMessage !== 'Migration réussie !'}
				>
					{resultMessage}
				</p>
			{/if}
		</div>

		<Dialog.Footer class="flex gap-2">
			<Button variant="outline" onclick={() => (modalOpen = false)} disabled={migrating}>
				Annuler
			</Button>
			<Button onclick={handleMigrate} disabled={migrating}>
				{#if migrating}
					<LoaderCircle class="size-4 animate-spin" />
				{/if}
				Migrer
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

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
