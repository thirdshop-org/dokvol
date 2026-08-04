<script lang="ts">
	import { onMount } from 'svelte';
	import { getBackupSchedules, getBackupTargets, getApplications, createBackupSchedule, updateBackupSchedule, deleteBackupSchedule } from '$lib/api';
	import type { BackupSchedule, BackupTarget, ApplicationVolumes } from '$lib/types';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { LoaderCircle, Plus, Trash2 } from '@lucide/svelte';
	import { errorMessage } from '$lib/utils/errors';
	import { t } from '$lib/i18n';

	let schedules = $state<BackupSchedule[]>([]);
	let targets = $state<BackupTarget[]>([]);
	let apps = $state<ApplicationVolumes[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let showForm = $state(false);
	let newApp = $state('');
	let newTarget = $state('');
	let newCron = $state('0 2 * * *');
	let newRetention = $state(7);
	let creating = $state(false);
	let createError = $state<string | null>(null);

	let toggling = $state<Record<string, boolean>>({});
	let deleting = $state<Record<string, boolean>>({});

	onMount(async () => {
		try {
			const [s, tg, a] = await Promise.all([
				getBackupSchedules(),
				getBackupTargets(),
				getApplications(),
			]);
			schedules = s;
			targets = tg;
			apps = a;
		} catch (e) {
			error = errorMessage(e);
		} finally {
			loading = false;
		}
	});

	async function handleCreate() {
		if (!newApp || !newTarget || !newCron) {
			createError = $t('backup.schedules.allFieldsRequired');
			return;
		}
		creating = true;
		createError = null;
		try {
			await createBackupSchedule({ target_id: newTarget, app_name: newApp, cron_expr: newCron, retention: newRetention });
			schedules = await getBackupSchedules();
			showForm = false;
			newApp = '';
			newTarget = '';
			newCron = '0 2 * * *';
			newRetention = 7;
		} catch (e) {
			createError = errorMessage(e);
		} finally {
			creating = false;
		}
	}

	async function toggleEnabled(sched: BackupSchedule) {
		toggling[sched.id] = true;
		try {
			await updateBackupSchedule(sched.id, { enabled: !sched.enabled });
			schedules = schedules.map(s => s.id === sched.id ? { ...s, enabled: !s.enabled } : s);
		} catch {
			// ignore
		} finally {
			toggling[sched.id] = false;
		}
	}

	async function handleDelete(id: string) {
		if (!confirm($t('backup.schedules.confirmDelete'))) return;
		deleting[id] = true;
		try {
			await deleteBackupSchedule(id);
			schedules = schedules.filter(s => s.id !== id);
		} catch (e) {
			alert(errorMessage(e));
		} finally {
			deleting[id] = false;
		}
	}
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight">{$t('backup.schedules.title')}</h1>
			<p class="text-muted-foreground">{$t('backup.schedules.description')}</p>
		</div>
		<Button onclick={() => (showForm = !showForm)}>
			<Plus class="size-4" />
			{showForm ? $t('backup.cancel') : $t('backup.schedules.newSchedule')}
		</Button>
	</div>

	{#if showForm}
		<Card.Root>
			<Card.Content class="space-y-4 pt-6">
				<h3 class="text-sm font-semibold">{$t('backup.schedules.createTitle')}</h3>
				<div class="grid gap-4 sm:grid-cols-2">
					<div class="space-y-2">
						<label for="sched-app" class="text-sm font-medium">{$t('backup.schedules.application')}</label>
						<select
							id="sched-app"
							class="border-input bg-background ring-offset-background focus-visible:ring-ring flex h-9 w-full rounded-md border px-3 py-1 text-sm shadow-xs focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none"
							bind:value={newApp}
							disabled={creating}
						>
							<option value="">{$t('backup.schedules.select')}</option>
							{#each apps as app (app.ContainerName)}
								<option value={app.ContainerName}>{app.ContainerName.replace(/^\//, '')}</option>
							{/each}
						</select>
					</div>
					<div class="space-y-2">
						<label for="sched-target" class="text-sm font-medium">{$t('backup.schedules.target')}</label>
						<select
							id="sched-target"
							class="border-input bg-background ring-offset-background focus-visible:ring-ring flex h-9 w-full rounded-md border px-3 py-1 text-sm shadow-xs focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none"
							bind:value={newTarget}
							disabled={creating}
						>
							<option value="">{$t('backup.schedules.select')}</option>
							{#each targets as tgt (tgt.id)}
								<option value={tgt.id}>{tgt.name} ({tgt.provider})</option>
							{/each}
						</select>
					</div>
					<div class="space-y-2">
						<label for="sched-cron" class="text-sm font-medium">{$t('backup.schedules.cronExpr')}</label>
						<Input id="sched-cron" bind:value={newCron} placeholder="0 2 * * *" disabled={creating} />
					</div>
					<div class="space-y-2">
						<label for="sched-retention" class="text-sm font-medium">{$t('backup.schedules.retention')}</label>
						<Input id="sched-retention" bind:value={newRetention} type="number" min="1" disabled={creating} />
					</div>
				</div>
				{#if createError}
					<p class="text-sm text-destructive">{createError}</p>
				{/if}
				<div class="flex justify-end">
					<Button onclick={handleCreate} disabled={creating || !newApp || !newTarget || !newCron}>
						{#if creating}<LoaderCircle class="size-4 animate-spin" />{/if}
						{$t('backup.schedules.create')}
					</Button>
				</div>
			</Card.Content>
		</Card.Root>
	{/if}

	{#if loading}
		<p class="text-muted-foreground">{$t('backup.loading')}</p>
	{:else if error}
		<p class="text-destructive">{error}</p>
	{:else if schedules.length === 0 && !showForm}
		<div class="rounded-lg border border-dashed p-12 text-center">
			<p class="text-muted-foreground">{$t('backup.schedules.empty')}</p>
		</div>
	{:else}
		<div class="rounded-lg border overflow-x-auto">
			<table class="w-full text-sm">
				<thead class="border-b bg-muted/50 text-muted-foreground">
					<tr>
						<th class="px-4 py-3 text-left font-medium">{$t('backup.schedules.table.application')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('backup.schedules.table.target')}</th>
						<th class="px-4 py-3 text-left font-medium">{$t('backup.schedules.table.cron')}</th>
						<th class="px-4 py-3 text-center font-medium">{$t('backup.schedules.table.retention')}</th>
						<th class="px-4 py-3 text-center font-medium">{$t('backup.schedules.table.enabled')}</th>
						<th class="px-4 py-3 text-center font-medium">{$t('backup.schedules.table.actions')}</th>
					</tr>
				</thead>
				<tbody>
					{#each schedules as sched (sched.id)}
						{@const target = targets.find(t => t.id === sched.target_id)}
						<tr class="border-b last:border-0 hover:bg-muted/30">
							<td class="px-4 py-3 font-medium">{sched.app_name}</td>
							<td class="px-4 py-3 text-muted-foreground">{target?.name || sched.target_id}</td>
							<td class="px-4 py-3 font-mono text-xs">{sched.cron_expr}</td>
							<td class="px-4 py-3 text-center">{sched.retention}d</td>
							<td class="px-4 py-3 text-center">
								<button
									onclick={() => toggleEnabled(sched)}
									disabled={toggling[sched.id]}
									class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
									class:bg-primary={sched.enabled}
									class:bg-input={!sched.enabled}
									role="switch"
									aria-checked={sched.enabled}
									aria-label={sched.enabled ? $t('backup.schedules.disableSchedule') : $t('backup.schedules.enableSchedule')}
								>
									<span
										class="pointer-events-none block size-4 rounded-full bg-white shadow-lg ring-0 transition-transform"
										class:translate-x-4={sched.enabled}
										class:translate-x-0={!sched.enabled}
									></span>
								</button>
							</td>
							<td class="px-4 py-3 text-center">
								<Button size="sm" variant="destructive" onclick={() => handleDelete(sched.id)} disabled={deleting[sched.id]}>
									{#if deleting[sched.id]}<LoaderCircle class="size-3 animate-spin" />{:else}<Trash2 class="size-3" />{/if}
								</Button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
