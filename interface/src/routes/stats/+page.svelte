<script lang="ts">
	import { onMount } from "svelte";
	import { t } from "$lib/i18n";
	import * as Card from "$lib/components/ui/card/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import StatsChart from "$lib/components/charts/StatsChart.svelte";
	import Sparkline from "$lib/components/charts/Sparkline.svelte";
	import { getDrives, getStatsDrive, getStatsVolume, getStatsApplication, getVolumes, getApplications, getStatsMigration } from "$lib/api";
	import { AreaChart, HardDrive, Layers, Box, History } from "@lucide/svelte";
	import type { MigrationStats } from "$lib/types";

	type RangeKey = "7d" | "30d" | "90d" | "all";

	let selectedRange = $state<RangeKey>("7d");
	let driveCharts = $state<{ mountpoint: string; device: string; data: { date: Date; value: number }[] }[]>([]);
	let volumeCharts = $state<{ name: string; app: string; total: number; data: { date: Date; value: number }[] }[]>([]);
	let appCharts = $state<{ name: string; data: { date: Date; value: number }[] }[]>([]);
	let migrationStats = $state<MigrationStats | null>(null);

	let drivesLoading = $state(true);
	let volumesLoading = $state(true);
	let appsLoading = $state(true);
	let migrationsLoading = $state(true);

	let abortController: AbortController | null = null;

	function rangeDays(key: RangeKey): number {
		if (key === "7d") return 7;
		if (key === "30d") return 30;
		if (key === "90d") return 90;
		return 365;
	}

	const dateRange = $derived.by(() => {
		const to = new Date();
		const from = new Date();
		from.setDate(from.getDate() - rangeDays(selectedRange));
		return [from, to] as [Date, Date];
	});

	function toISO(daysAgo: number): string {
		const d = new Date();
		d.setDate(d.getDate() - daysAgo);
		return d.toISOString();
	}

	async function loadData() {
		abortController?.abort();
		const ac = new AbortController();
		abortController = ac;
		const signal = ac.signal;

		const from = toISO(rangeDays(selectedRange));

		migrationsLoading = true;
		drivesLoading = driveCharts.length === 0;
		volumesLoading = volumeCharts.length === 0;
		appsLoading = appCharts.length === 0;

		getStatsMigration(signal)
			.then(m => { if (!signal.aborted) { migrationStats = m; migrationsLoading = false; }})
			.catch(() => { if (!signal.aborted) migrationsLoading = false; });

		fetchDriveCharts(from, signal)
			.then(d => { if (!signal.aborted) { driveCharts = d; drivesLoading = false; }})
			.catch(() => { if (!signal.aborted) drivesLoading = false; });

		fetchVolumeCharts(from, signal)
			.then(v => { if (!signal.aborted) { volumeCharts = v; volumesLoading = false; }})
			.catch(() => { if (!signal.aborted) volumesLoading = false; });

		fetchAppCharts(from, signal)
			.then(a => { if (!signal.aborted) { appCharts = a; appsLoading = false; }})
			.catch(() => { if (!signal.aborted) appsLoading = false; });
	}

	async function fetchDriveCharts(from: string, signal: AbortSignal) {
		const drives = await getDrives(signal);
		const results = await Promise.all(
			drives.map(d =>
				getStatsDrive(d.mountpoint, from, undefined, signal)
					.then(rows => ({
						mountpoint: d.mountpoint,
						device: d.device,
						data: rows.map(r => ({ date: new Date(r.captured_at), value: r.used_bytes })),
					}))
					.catch(() => null)
			)
		);
		return results.filter(Boolean) as NonNullable<(typeof results)[number]>[];
	}

	async function fetchVolumeCharts(from: string, signal: AbortSignal) {
		try {
			const volumes = await getVolumes(signal);
			const volNames = [...new Set(volumes.map(v => v.Name || v.Source))].slice(0, 10);
			const results = await Promise.all(
				volNames.map(async name => {
					try {
						const rows = await getStatsVolume(name, from, undefined, signal);
						const app = volumes.find(v => (v.Name || v.Source) === name);
						return {
							name,
							app: app?.ContainerName ?? "",
							total: rows.length > 0 ? rows[rows.length - 1].total_bytes : 0,
							data: rows.map(r => ({ date: new Date(r.captured_at), value: r.total_bytes })),
						};
					} catch {
						return null;
					}
				})
			);
			const filtered = results.filter(Boolean) as NonNullable<(typeof results)[number]>[];
			filtered.sort((a, b) => b.total - a.total);
			return filtered.slice(0, 10);
		} catch {
			return [];
		}
	}

	async function fetchAppCharts(from: string, signal: AbortSignal) {
		try {
			const apps = await getApplications(signal);
			const results = await Promise.all(
				apps.map(a =>
					getStatsApplication(a.ContainerName, from, undefined, signal)
						.then(rows => ({
							name: a.ContainerName,
							data: rows.map(r => ({ date: new Date(r.captured_at), value: r.total_bytes ?? 0 })),
						}))
						.catch(() => null)
				)
			);
			return results.filter(Boolean) as NonNullable<(typeof results)[number]>[];
		} catch {
			return [];
		}
	}

	onMount(() => {
		loadData();
		return () => {
			abortController?.abort();
		};
	});

	function formatDuration(ms: number): string {
		if (ms < 1000) return `${ms}ms`;
		const s = Math.floor(ms / 1000);
		if (s < 60) return `${s}s`;
		const m = Math.floor(s / 60);
		const sec = s % 60;
		if (m < 60) return `${m}m ${sec}s`;
		const h = Math.floor(m / 60);
		const min = m % 60;
		return `${h}h ${min}m`;
	}

	function formatBytes(v: number): string {
		if (v === 0) return "0 B";
		const units = ["B", "KB", "MB", "GB", "TB"];
		const i = Math.floor(Math.log(v) / Math.log(1024));
		return `${(v / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
	}
</script>

<div class="space-y-6">
	<div class="flex items-start justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight">{$t("stats.title")}</h1>
			<p class="text-muted-foreground">{$t("stats.description")}</p>
		</div>
		<div class="flex">
			{#each ["7d", "30d", "90d", "all"] as range (range)}
				<button
					data-active={selectedRange === range}
					class="data-[active=true]:bg-muted/50 relative z-30 flex flex-1 flex-col justify-center gap-1 border-t px-4 py-3 text-start even:border-s sm:border-s sm:border-t-0 sm:px-6 sm:py-4"
					onclick={() => { selectedRange = range as RangeKey; loadData(); }}
				>
					<span class="text-xs text-muted-foreground">{$t(`stats.range.${range}`)}</span>
				</button>
			{/each}
		</div>
	</div>

	<!-- Migration stats -->
	{#if migrationStats || migrationsLoading}
		<div class="space-y-4">
			<h2 class="text-lg font-semibold flex items-center gap-2">
				<History class="size-5" /> {$t("stats.migrations")}
			</h2>
			<div class="grid gap-4 sm:grid-cols-3">
				{#if migrationsLoading && !migrationStats}
					{#each [1, 2, 3] as _}
						<Card.Root>
							<Card.Header class="pb-2">
								<Skeleton class="h-8 w-20 mb-1" />
								<Skeleton class="h-4 w-32" />
							</Card.Header>
						</Card.Root>
					{/each}
				{:else if migrationStats}
					<Card.Root>
						<Card.Header class="pb-2">
							<Card.Title class="text-2xl font-bold">{migrationStats.total_count}</Card.Title>
							<Card.Description>{$t("stats.migrationTotal")}</Card.Description>
						</Card.Header>
					</Card.Root>
					<Card.Root>
						<Card.Header class="pb-2">
							<Card.Title class="text-2xl font-bold text-emerald-600">{migrationStats.completed_count}</Card.Title>
							<Card.Description>{$t("stats.migrationCompleted")}</Card.Description>
						</Card.Header>
					</Card.Root>
					<Card.Root>
						<Card.Header class="pb-2">
							<Card.Title class="text-2xl font-bold text-red-600">{migrationStats.failed_count}</Card.Title>
							<Card.Description>{$t("stats.migrationFailed")}</Card.Description>
						</Card.Header>
					</Card.Root>
				{/if}
			</div>
			{#if !migrationsLoading && migrationStats}
				<div class="grid gap-4 sm:grid-cols-3">
					<Card.Root>
						<Card.Header class="pb-2">
							<Card.Title class="text-2xl font-bold">{formatBytes(migrationStats.total_bytes_moved)}</Card.Title>
							<Card.Description>{$t("stats.migrationBytes")}</Card.Description>
						</Card.Header>
					</Card.Root>
					<Card.Root>
						<Card.Header class="pb-2">
							<Card.Title class="text-2xl font-bold">{formatDuration(migrationStats.total_duration_ms)}</Card.Title>
							<Card.Description>{$t("stats.migrationDuration")}</Card.Description>
						</Card.Header>
					</Card.Root>
					<Card.Root>
						<Card.Header class="pb-2">
							<Card.Title class="text-2xl font-bold">{migrationStats.unique_apps}</Card.Title>
							<Card.Description>{$t("stats.migrationApps")}</Card.Description>
						</Card.Header>
					</Card.Root>
				</div>
			{/if}
		</div>
	{/if}

	<!-- Drives section -->
	<div class="space-y-4">
		<h2 class="text-lg font-semibold flex items-center gap-2">
			<HardDrive class="size-5" /> {$t("stats.drives")}
		</h2>
		<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
			{#if drivesLoading && driveCharts.length === 0}
				{#each [1, 2, 3] as _}
					<Card.Root>
						<Card.Content>
							<Skeleton class="w-full" style="height: 180px;" />
						</Card.Content>
					</Card.Root>
				{/each}
			{:else}
				{#each driveCharts as chart}
					<a href="/stats/drives?mountpoint={encodeURIComponent(chart.mountpoint)}" class="block">
						<StatsChart
							title={chart.mountpoint}
							description={chart.device}
							data={chart.data}
							height="180px"
							xDomain={dateRange}
							formatX={(v: Date) => v.toLocaleDateString(undefined, { month: "short", day: "numeric" })}
							formatY={(v: number) => formatBytes(v)}
						/>
					</a>
				{/each}
			{/if}
		</div>
	</div>

	<!-- Volumes section -->
	<div class="space-y-4">
		<h2 class="text-lg font-semibold flex items-center gap-2">
			<Layers class="size-5" /> {$t("stats.topVolumes")}
		</h2>
		<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
			{#if volumesLoading && volumeCharts.length === 0}
				{#each [1, 2, 3] as _}
					<Card.Root>
						<Card.Header>
							<Skeleton class="h-5 w-40 mb-2" />
							<Skeleton class="h-4 w-28" />
						</Card.Header>
						<Card.Content>
							<Skeleton class="w-full" style="height: 40px;" />
						</Card.Content>
					</Card.Root>
				{/each}
			{:else}
				{#each volumeCharts as chart}
					<a href="/stats/volumes?name={encodeURIComponent(chart.name)}" class="block">
						<Card.Root>
							<Card.Header>
								<Card.Title class="truncate">{chart.name}</Card.Title>
								<Card.Description>
									{chart.app} — {formatBytes(chart.total)}
								</Card.Description>
							</Card.Header>
							<Card.Content>
								<Sparkline data={chart.data.map(d => ({ value: d.value }))} width={200} height={40} color="var(--chart-2)" />
							</Card.Content>
						</Card.Root>
					</a>
				{/each}
			{/if}
		</div>
	</div>

	<!-- Applications section -->
	<div class="space-y-4">
		<h2 class="text-lg font-semibold flex items-center gap-2">
			<Box class="size-5" /> {$t("stats.applications")}
		</h2>
		<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
			{#if appsLoading && appCharts.length === 0}
				{#each [1, 2, 3] as _}
					<Card.Root>
						<Card.Content>
							<Skeleton class="w-full" style="height: 180px;" />
						</Card.Content>
					</Card.Root>
				{/each}
			{:else}
				{#each appCharts as chart}
					<a href="/stats/applications?name={encodeURIComponent(chart.name)}" class="block">
						<StatsChart
							title={chart.name}
							data={chart.data}
							height="180px"
							color="var(--chart-3)"
							xDomain={dateRange}
							formatX={(v: Date) => v.toLocaleDateString(undefined, { month: "short", day: "numeric" })}
							formatY={(v: number) => formatBytes(v)}
						/>
					</a>
				{/each}
			{/if}
		</div>
	</div>
</div>
