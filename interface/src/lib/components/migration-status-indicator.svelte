<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { getActiveMigrations } from '$lib/api';
	import type { MigrationJob } from '$lib/types';
	import { t } from '$lib/i18n';
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';

	let jobs = $state<MigrationJob[]>([]);
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	let activeJobs = $derived(jobs.filter(j => j.status === 'pending' || j.status === 'running'));

	function stepLabel(step: string): string {
		try { return $t(`step.${step}`); }
		catch { return step; }
	}

	async function poll() {
		try {
			jobs = await getActiveMigrations();
		} catch {
			// silently ignore
		}
	}

	onMount(() => {
		poll();
		pollTimer = setInterval(poll, 5000);
	});

	onDestroy(() => {
		if (pollTimer) {
			clearInterval(pollTimer);
		}
	});
</script>

{#if activeJobs.length > 0}
	<Tooltip.Root>
		<Tooltip.Trigger>
			<button
				class="relative flex items-center justify-center size-8 rounded-md hover:bg-accent hover:text-accent-foreground transition-colors"
				aria-label={$t('migration.indicator.aria', { n: String(activeJobs.length) })}
			>
				<LoaderCircle class="size-4 animate-spin text-blue-500 dark:text-blue-400" />
				<span class="absolute -top-0.5 -right-0.5 flex size-4 items-center justify-center rounded-full bg-blue-500 text-[10px] font-bold text-white leading-none">
					{activeJobs.length}
				</span>
			</button>
		</Tooltip.Trigger>
		<Tooltip.Content side="bottom" align="end" sideOffset={6} class="w-72">
			<div class="space-y-3">
				<p class="text-sm font-medium">
					{$t('migration.indicator.active', { n: String(activeJobs.length) })}
				</p>
				{#each activeJobs as job}
					<div class="border-t pt-3 first:border-t-0 first:pt-0">
						<div class="flex items-center justify-between gap-2">
							<a
								href="/applications/{job.app_name.replace(/^\//, '')}"
								class="text-sm font-medium hover:underline truncate"
							>
								{job.app_name.replace(/^\//, '')}
							</a>
							<Badge variant="running" class="text-[10px] px-1.5 py-0 shrink-0">
								{job.status}
							</Badge>
						</div>
						<div class="mt-2 space-y-1.5">
							{#each job.volumes as vol}
								<div>
									<div class="flex items-center justify-between gap-2 text-xs text-muted-foreground">
										<span class="truncate min-w-0">{vol.volume_name}</span>
										<span class="shrink-0">{stepLabel(vol.step)}</span>
									</div>
									{#if vol.total_bytes > 0 && (vol.step === 'syncing' || vol.step === 'verifying')}
									<div class="mt-0.5 w-full h-1 bg-muted rounded-full overflow-hidden">
										<div
											class="h-full bg-blue-500 rounded-full transition-all"
											style="width: {Math.min(100, Math.round((vol.transferred_bytes / vol.total_bytes) * 100))}%"
										></div>
									</div>
								{/if}
								</div>
							{/each}
						</div>
					</div>
				{/each}
			</div>
		</Tooltip.Content>
	</Tooltip.Root>
{/if}
