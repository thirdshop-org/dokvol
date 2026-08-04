<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { getBackupJob, getBackupTargets, restoreBackup } from '$lib/api';
	import type { BackupVolumeProgress, BackupTarget } from '$lib/types';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { LoaderCircle, ArrowLeft, Download } from '@lucide/svelte';
	import { feedbackBoxClass } from '$lib/utils/status';
	import { formatBytes } from '$lib/utils/format';
	import { errorMessage } from '$lib/utils/errors';
	import { t } from '$lib/i18n';

	let jobId = $state('');
	let status = $state('');
	let appName = $state('');
	let volumes = $state<BackupVolumeProgress[]>([]);
	let targets = $state<BackupTarget[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let selectedTargetId = $state('');
	let destMountpoint = $state('');
	let restoring = $state(false);
	let restoreResult = $state<string | null>(null);
	let restoreSuccess = $state(false);

	onMount(async () => {
		jobId = $page.params.jobId as string;
		try {
			const [job, ts] = await Promise.all([
				getBackupJob(jobId),
				getBackupTargets(),
			]);
			status = job.status;
			volumes = job.volumes;
			targets = ts;
			appName = job.id; // API doesn't return app_name in getBackupJob, infer from context
		} catch (e) {
			error = errorMessage(e);
		} finally {
			loading = false;
		}
	});

	async function handleRestore() {
		if (!selectedTargetId) {
			restoreResult = $t('backup.restore.selectTargetRequired');
			restoreSuccess = false;
			return;
		}
		restoring = true;
		restoreResult = null;
		try {
			const res = await restoreBackup({
				job_id: jobId,
				target_id: selectedTargetId,
				app_name: appName,
				dest_mountpoint: destMountpoint || undefined,
			});
			restoreResult = $t('backup.restore.restoreStarted', { id: res.job_id });
			restoreSuccess = true;
		} catch (e) {
			restoreResult = errorMessage(e);
			restoreSuccess = false;
		} finally {
			restoring = false;
		}
	}


</script>

<div class="space-y-6">
	<div>
		<a href="/backup/jobs/{jobId}" class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
			<ArrowLeft class="size-4" />
			{$t('backup.restore.backToJob')}
		</a>
	</div>

	{#if loading}
		<p class="text-muted-foreground">{$t('backup.loading')}</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else if status !== 'completed'}
		<div class="rounded-lg border border-dashed p-12 text-center">
			<p class="text-muted-foreground">{$t('backup.restore.onlyCompleted')}</p>
			<a href="/backup/jobs/{jobId}" class="mt-2 inline-block">
				<Button variant="outline">{$t('backup.restore.backToJob')}</Button>
			</a>
		</div>
	{:else}
		<div>
			<h1 class="text-2xl font-bold tracking-tight">{$t('backup.restore.title')}</h1>
			<p class="text-muted-foreground">{$t('backup.restore.job')}: <span class="font-mono">{jobId}</span></p>
		</div>

		<div class="grid gap-6 md:grid-cols-2">
			<Card.Root>
				<Card.Header>
					<Card.Title>{$t('backup.restore.jobDetails')}</Card.Title>
				</Card.Header>
				<Card.Content>
					<div class="space-y-3">
						<div class="rounded-lg border p-3">
							<p class="text-xs text-muted-foreground">{$t('backup.restore.status')}</p>
							<p class="font-medium">{status}</p>
						</div>
						<div class="rounded-lg border p-3">
							<p class="text-xs text-muted-foreground">{$t('backup.restore.volumes')}</p>
							<p class="font-medium">{volumes.length}</p>
						</div>
						<div class="space-y-2">
							{#each volumes as vol, i (vol.VolumeName || i)}
								<div class="rounded-lg border p-3 text-sm">
									<p class="font-medium">{vol.VolumeName}</p>
									<p class="text-xs text-muted-foreground font-mono">{vol.BackupPath}</p>
									<p class="text-xs text-muted-foreground">{formatBytes(vol.TotalBytes)}</p>
								</div>
							{/each}
						</div>
					</div>
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header>
					<Card.Title>{$t('backup.restore.restoreOptions')}</Card.Title>
				</Card.Header>
				<Card.Content class="space-y-4">
					<div class="space-y-2">
						<label for="restore-target" class="text-sm font-medium">{$t('backup.restore.target')}</label>
						<select
							id="restore-target"
							class="border-input bg-background ring-offset-background focus-visible:ring-ring flex h-9 w-full rounded-md border px-3 py-1 text-sm shadow-xs focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none"
							bind:value={selectedTargetId}
							disabled={restoring}
						>
							<option value="">{$t('backup.restore.selectTarget')}</option>
							{#each targets as tgt (tgt.id)}
								<option value={tgt.id}>{tgt.name} ({tgt.provider})</option>
							{/each}
						</select>
					</div>

					<div class="space-y-2">
						<label for="restore-dest" class="text-sm font-medium">{$t('backup.restore.destMountpoint')}</label>
						<Input id="restore-dest" bind:value={destMountpoint} placeholder="/mnt/restore" disabled={restoring} />
						<p class="text-xs text-muted-foreground">{$t('backup.restore.destHint')}</p>
					</div>

					{#if restoreResult}
						<div class="rounded-lg border p-3 text-sm {feedbackBoxClass(restoreSuccess)}">
							{restoreResult}
						</div>
					{/if}

					<Button onclick={handleRestore} disabled={restoring || !selectedTargetId} class="w-full">
						{#if restoring}
							<LoaderCircle class="size-4 animate-spin" />
						{:else}
							<Download class="size-4" />
						{/if}
						{restoring ? $t('backup.restore.restoring') : $t('backup.restore.confirmRestore')}
					</Button>
				</Card.Content>
			</Card.Root>
		</div>
	{/if}
</div>
