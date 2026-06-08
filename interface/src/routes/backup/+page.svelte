<script lang="ts">
	import { onMount } from 'svelte';
	import { getBackupTargets, getBackupJobs, getBackupSchedules } from '$lib/api';
	import type { BackupTarget, BackupJob, BackupSchedule } from '$lib/types';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Database, ListTodo, Clock, Plus } from '@lucide/svelte';

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
			error = e instanceof Error ? e.message : 'Failed to load backup data';
		} finally {
			loading = false;
		}
	});
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight">Backup</h1>
			<p class="text-muted-foreground">Manage backup targets, jobs, and schedules</p>
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
	{:else}
		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			<a href="/backup/targets" class="block">
				<Card.Root class="transition-colors hover:bg-accent">
					<Card.Header class="flex flex-row items-center justify-between pb-2">
						<Card.Title class="text-sm font-medium">Targets</Card.Title>
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
						<Card.Title class="text-sm font-medium">Recent Jobs</Card.Title>
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
						<Card.Title class="text-sm font-medium">Schedules</Card.Title>
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
					<Card.Title>Recent Jobs</Card.Title>
				</Card.Header>
				<Card.Content>
					{#if recentJobs.length === 0}
						<p class="text-sm text-muted-foreground">No backup jobs yet</p>
					{:else}
						<div class="space-y-2">
							{#each recentJobs as job (job.id)}
								<a href="/backup/jobs/{job.id}" class="flex items-center justify-between rounded-lg border p-3 text-sm transition-colors hover:bg-accent">
									<div>
										<p class="font-medium">{job.app_name}</p>
										<p class="text-xs text-muted-foreground">{job.started_at ? new Date(job.started_at).toLocaleString() : '—'}</p>
									</div>
									<span class="rounded-full px-2 py-0.5 text-xs font-medium" class:bg-green-100:text-green-800:dark:bg-green-900:dark:text-green-100={job.status === 'completed'} class:bg-red-100:text-red-800:dark:bg-red-900:dark:text-red-100={job.status === 'failed'} class:bg-blue-100:text-blue-800:dark:bg-blue-900:dark:text-blue-100={job.status === 'running'} class:bg-gray-100:text-gray-800:dark:bg-gray-800:dark:text-gray-100={job.status === 'pending'}>{job.status}</span>
								</a>
							{/each}
						</div>
					{/if}
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header>
					<Card.Title>Schedules</Card.Title>
				</Card.Header>
				<Card.Content>
					{#if schedules.length === 0}
						<p class="text-sm text-muted-foreground">No schedules configured</p>
					{:else}
						<div class="space-y-2">
							{#each schedules as sched (sched.id)}
								<div class="flex items-center justify-between rounded-lg border p-3 text-sm">
									<div>
										<p class="font-medium">{sched.app_name}</p>
										<p class="text-xs text-muted-foreground font-mono">{sched.cron_expr}</p>
									</div>
									<span class="rounded-full px-2 py-0.5 text-xs font-medium" class:bg-green-100:text-green-800:dark:bg-green-900:dark:text-green-100={sched.enabled} class:bg-gray-100:text-gray-800:dark:bg-gray-800:dark:text-gray-100={!sched.enabled}>{sched.enabled ? 'Enabled' : 'Disabled'}</span>
								</div>
							{/each}
						</div>
					{/if}
				</Card.Content>
			</Card.Root>
		</div>
	{/if}
</div>
