<script lang="ts">
	import { onMount } from 'svelte';
	import { getBackupTargets, getBackupJobs, getBackupSchedules } from '$lib/api';
	import type { BackupTarget, BackupJob, BackupSchedule } from '$lib/types';
	import { t } from '$lib/i18n';
	import { errorMessage } from '$lib/utils/errors';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Database, ListTodo, Clock, Plus } from '@lucide/svelte';
	import { statusBadgeClass, enabledBadgeClass } from '$lib/utils/status';

	let targets = $state<BackupTarget[]>([]);
	let recentJobs = $state<BackupJob[]>([]);
	let schedules = $state<BackupSchedule[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(async () => {
		try {
			const [t, j, s] = await Promise.all([
				getBackupTargets(),
				getBackupJobs({ limit: 10 }),
				getBackupSchedules(),
			]);
			targets = t;
			recentJobs = j.jobs;
			schedules = s;
		} catch (e) {
			error = errorMessage(e);
		} finally {
			loading = false;
		}
	});
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight">{$t('backup.title')}</h1>
			<p class="text-muted-foreground">{$t('backup.description')}</p>
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
	{:else}
		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			<a href="/backup/targets" class="block">
				<Card.Root class="transition-colors hover:bg-accent">
					<Card.Header class="flex flex-row items-center justify-between pb-2">
						<Card.Title class="text-sm font-medium">{$t('backup.cards.targets')}</Card.Title>
						<Database class="size-4 text-muted-foreground" />
					</Card.Header>
					<Card.Content>
						<p class="text-3xl font-bold">{targets.length}</p>
					</Card.Content>
				</Card.Root>
			</a>
			<a href="/backup/jobs" class="block">
				<Card.Root class="transition-colors hover:bg-accent">
					<Card.Header class="flex flex-row items-center justify-between pb-2">
						<Card.Title class="text-sm font-medium">{$t('backup.cards.recentJobs')}</Card.Title>
						<ListTodo class="size-4 text-muted-foreground" />
					</Card.Header>
					<Card.Content>
						<p class="text-3xl font-bold">{recentJobs.length}</p>
					</Card.Content>
				</Card.Root>
			</a>
			<a href="/backup/schedules" class="block">
				<Card.Root class="transition-colors hover:bg-accent">
					<Card.Header class="flex flex-row items-center justify-between pb-2">
						<Card.Title class="text-sm font-medium">{$t('backup.cards.schedules')}</Card.Title>
						<Clock class="size-4 text-muted-foreground" />
					</Card.Header>
					<Card.Content>
						<p class="text-3xl font-bold">{schedules.length}</p>
					</Card.Content>
				</Card.Root>
			</a>
		</div>

		<div class="grid gap-6 md:grid-cols-2">
			<Card.Root>
				<Card.Header>
					<Card.Title>{$t('backup.cards.recentJobs')}</Card.Title>
				</Card.Header>
				<Card.Content>
					{#if recentJobs.length === 0}
						<p class="text-sm text-muted-foreground">{$t('backup.noJobs')}</p>
					{:else}
						<div class="space-y-2">
							{#each recentJobs as job (job.id)}
								<a href="/backup/jobs/{job.id}" class="flex items-center justify-between rounded-lg border p-3 text-sm transition-colors hover:bg-accent">
									<div>
										<p class="font-medium">{job.app_name}</p>
										<p class="text-xs text-muted-foreground">{job.started_at ? new Date(job.started_at).toLocaleString() : '—'}</p>
									</div>
									<span class="rounded-full px-2 py-0.5 text-xs font-medium {statusBadgeClass(job.status)}">{job.status}</span>
								</a>
							{/each}
						</div>
					{/if}
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header>
					<Card.Title>{$t('backup.cards.schedules')}</Card.Title>
				</Card.Header>
				<Card.Content>
					{#if schedules.length === 0}
						<p class="text-sm text-muted-foreground">{$t('backup.noSchedules')}</p>
					{:else}
						<div class="space-y-2">
							{#each schedules as sched (sched.id)}
								<div class="flex items-center justify-between rounded-lg border p-3 text-sm">
									<div>
										<p class="font-medium">{sched.app_name}</p>
										<p class="text-xs text-muted-foreground font-mono">{sched.cron_expr}</p>
									</div>
									<span class="rounded-full px-2 py-0.5 text-xs font-medium {enabledBadgeClass(sched.enabled)}">{sched.enabled ? $t('backup.enabled') : $t('backup.disabled')}</span>
								</div>
							{/each}
						</div>
					{/if}
				</Card.Content>
			</Card.Root>
		</div>
	{/if}
</div>
