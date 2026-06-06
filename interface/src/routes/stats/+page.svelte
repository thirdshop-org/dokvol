<script lang="ts">
	import { onMount } from "svelte";
	import { t } from "$lib/i18n";
	import * as Card from "$lib/components/ui/card/index.js";
	import StatsChart from "$lib/components/charts/StatsChart.svelte";
	import Sparkline from "$lib/components/charts/Sparkline.svelte";
	import { getDrives, getStatsDrive, getStatsVolume, getStatsApplication, getVolumes, getApplications, getStatsMigration } from "$lib/api";
	import { AreaChart, HardDrive, Layers, Box, History } from "@lucide/svelte";
	import type { MigrationStats } from "$lib/types";

	type RangeKey = "7d" | "30d" | "90d" | "all";

	let ready = $state(false);
	let selectedRange = $state<RangeKey>("7d");
	let driveCharts = $state<{ mountpoint: string; device: string; data: { date: Date; value: number }[] }[]>([]);
	let volumeCharts = $state<{ name: string; app: string; total: number; data: { date: Date; value: number }[] }[]>([]);
	let appCharts = $state<{ name: string; data: { date: Date; value: number }[] }[]>([]);
	let migrationStats = $state<MigrationStats | null>(null);

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
		const from = toISO(rangeDays(selectedRange));
		const [d, v, a, m] = await Promise.all([
			fetchDriveCharts(from),
			fetchVolumeCharts(from),
			fetchAppCharts(from),
			getStatsMigration().catch(() => null),
		]);
		driveCharts = d;
		volumeCharts = v;
		appCharts = a;
		migrationStats = m;
	}

	async function fetchDriveCharts(from: string) {
		const drives = await getDrives();
		const results = await Promise.all(
			drives.map(d =>
				getStatsDrive(d.mountpoint, from)
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

	async function fetchVolumeCharts(from: string) {
		try {
			const volumes = await getVolumes();
			const volNames = [...new Set(volumes.map(v => v.Name || v.Source))].slice(0, 10);
			const results = await Promise.all(
				volNames.map(async name => {
					try {
						const rows = await getStatsVolume(name, from);
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

	async function fetchAppCharts(from: string) {
		try {
			const apps = await getApplications();
			const results = await Promise.all(
				apps.map(a =>
					getStatsApplication(a.ContainerName, from)
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

	onMount(async () => {
		try {
			await loadData();
		} catch {
			// individual fetch errors already handled per-chart
		} finally {
			ready = true;
		}
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

	{#if !ready}
		<p class="text-muted-foreground">{$t("stats.loading")}</p>
	{:else}

		<!-- Migration stats -->
		{#if migrationStats}
			<div class="space-y-4">
				<h2 class="text-lg font-semibold flex items-center gap-2">
					<History class="size-5" /> {$t("stats.migrations")}
				</h2>
				<div class="grid gap-4 sm:grid-cols-3">
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
				</div>
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
			</div>
		{/if}

		<!-- Drives section -->
		<div class="space-y-4">
			<h2 class="text-lg font-semibold flex items-center gap-2">
				<HardDrive class="size-5" /> {$t("stats.drives")}
			</h2>
			<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
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
			</div>
		</div>

		<!-- Volumes section -->
		<div class="space-y-4">
			<h2 class="text-lg font-semibold flex items-center gap-2">
				<Layers class="size-5" /> {$t("stats.topVolumes")}
			</h2>
			<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
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
			</div>
		</div>

		<!-- Applications section -->
		<div class="space-y-4">
			<h2 class="text-lg font-semibold flex items-center gap-2">
				<Box class="size-5" /> {$t("stats.applications")}
			</h2>
			<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
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
			</div>
		</div>

	{/if}
</div>
