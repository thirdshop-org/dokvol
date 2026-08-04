<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { getBackupTargets, getApplications, deleteBackupTarget, testBackupTarget, listBackupsOnTarget, runBackup } from '$lib/api';
	import type { BackupTarget, BackupListEntry, ApplicationVolumes } from '$lib/types';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { LoaderCircle, Plug, Trash2, ArrowLeft, Play, Database } from '@lucide/svelte';
	import { feedbackBoxClass } from '$lib/utils/status';
	import { formatBytes } from '$lib/utils/format';
	import { errorMessage } from '$lib/utils/errors';
	import { t } from '$lib/i18n';

	let target = $state<BackupTarget | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let testing = $state(false);
	let testMsg = $state<string | null>(null);
	let testSuccess = $state(false);

	let backups = $state<BackupListEntry[]>([]);
	let backupsLoading = $state(false);

	let apps = $state<ApplicationVolumes[]>([]);
	let selectedApp = $state('');
	let running = $state(false);
	let runMsg = $state<string | null>(null);

	onMount(async () => {
		try {
			const all = await getBackupTargets();
			const found = all.find(t => t.id === $page.params.id);
			if (!found) {
				error = $t('backup.targetDetail.notFound');
			} else {
				target = found;
			}
		} catch (e) {
			error = errorMessage(e);
		} finally {
			loading = false;
		}
	});

	async function handleTest() {
		if (!target) return;
		testing = true;
		testMsg = null;
		try {
			const res = await testBackupTarget(target.id);
			testMsg = res.message;
			testSuccess = res.success;
		} catch (e) {
			testMsg = errorMessage(e);
			testSuccess = false;
		} finally {
			testing = false;
		}
	}

	async function handleDelete() {
		if (!target || !confirm($t('backup.targetDetail.confirmDelete'))) return;
		try {
			await deleteBackupTarget(target.id);
			goto('/backup/targets');
		} catch (e) {
			alert(errorMessage(e));
		}
	}

	async function loadBackups() {
		if (!target || !selectedApp) return;
		backupsLoading = true;
		try {
			backups = await listBackupsOnTarget(target.id, selectedApp);
		} catch {
			backups = [];
		} finally {
			backupsLoading = false;
		}
	}

	async function handleRunBackup() {
		if (!target || !selectedApp) return;
		running = true;
		runMsg = null;
		try {
			const res = await runBackup(selectedApp, target.id);
			runMsg = $t('backup.targetDetail.backupStarted', { id: res.job_id });
		} catch (e) {
			runMsg = errorMessage(e);
		} finally {
			running = false;
		}
	}

	function loadApps() {
		getApplications().then(a => apps = a).catch(() => {});
	}

	$effect(() => {
		if (target) loadApps();
	});

	$effect(() => {
		if (target && selectedApp) loadBackups();
	});


</script>

<div class="space-y-6">
	<div>
		<a href="/backup/targets" class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
			<ArrowLeft class="size-4" />
			{$t('backup.targetDetail.backToTargets')}
		</a>
	</div>

	{#if loading}
		<p class="text-muted-foreground">{$t('backup.loading')}</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else if target}
		<div class="flex items-center justify-between">
			<div>
				<h1 class="text-2xl font-bold tracking-tight">{target.name}</h1>
				<p class="text-muted-foreground">
					<span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800 dark:bg-blue-900 dark:text-blue-100">{target.provider.toUpperCase()}</span>
					<span class="ml-2">{$t('backup.targetDetail.created')} {new Date(target.created_at).toLocaleString()}</span>
				</p>
			</div>
			<div class="flex gap-2">
				<Button variant="outline" onclick={handleTest} disabled={testing}>
					{#if testing}<LoaderCircle class="size-4 animate-spin" />{/if}
					<Plug class="size-4" />
					{$t('backup.targetDetail.testConnection')}
				</Button>
				<Button variant="destructive" onclick={handleDelete}>
					<Trash2 class="size-4" />
					{$t('backup.targetDetail.delete')}
				</Button>
			</div>
		</div>

		{#if testMsg}
			<div class="rounded-lg border p-3 text-sm {feedbackBoxClass(testSuccess)}">
				{testMsg}
			</div>
		{/if}

		<div class="grid gap-6 md:grid-cols-2">
			<Card.Root>
				<Card.Header>
					<Card.Title>{$t('backup.targetDetail.triggerBackup')}</Card.Title>
				</Card.Header>
				<Card.Content class="space-y-3">
					<div class="space-y-2">
						<label for="trigger-app" class="text-sm font-medium">{$t('backup.targetDetail.application')}</label>
						<select
							id="trigger-app"
							class="border-input bg-background ring-offset-background focus-visible:ring-ring flex h-9 w-full rounded-md border px-3 py-1 text-sm shadow-xs focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none"
							bind:value={selectedApp}
						>
							<option value="">{$t('backup.targetDetail.selectApp')}</option>
							{#each apps as app (app.ContainerName)}
								<option value={app.ContainerName}>{app.ContainerName.replace(/^\//, '')}</option>
							{/each}
						</select>
					</div>
					<Button onclick={handleRunBackup} disabled={running || !selectedApp}>
						{#if running}<LoaderCircle class="size-4 animate-spin" />{/if}
						<Play class="size-4" />
						{$t('backup.targetDetail.runBackup')}
					</Button>
					{#if runMsg}
						<p class="text-sm text-muted-foreground">{runMsg}</p>
					{/if}
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header>
					<Card.Title>{$t('backup.targetDetail.availableBackups')}</Card.Title>
				</Card.Header>
				<Card.Content>
					{#if !selectedApp}
						<p class="text-sm text-muted-foreground">{$t('backup.targetDetail.selectAppHint')}</p>
					{:else if backupsLoading}
						<p class="text-sm text-muted-foreground">{$t('backup.targetDetail.loadingBackups')}</p>
					{:else if backups.length === 0}
						<p class="text-sm text-muted-foreground">{$t('backup.targetDetail.noBackups')}</p>
					{:else}
						<div class="space-y-2">
							{#each backups as entry (entry.path)}
								<div class="flex items-center justify-between rounded-lg border p-3 text-sm">
									<div class="min-w-0">
										<p class="font-medium truncate">{entry.name}</p>
										<p class="text-xs text-muted-foreground">{formatBytes(entry.size)} — {new Date(entry.modified_at).toLocaleString()}</p>
									</div>
								</div>
							{/each}
						</div>
					{/if}
				</Card.Content>
			</Card.Root>
		</div>
	{/if}
</div>
