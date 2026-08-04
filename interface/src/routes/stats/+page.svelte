<script lang="ts">
	import { onMount, onDestroy } from "svelte";
	import { page } from "$app/stores";
	import { goto } from "$app/navigation";
	import { t } from "$lib/i18n";
	import * as Card from "$lib/components/ui/card/index.js";
	import * as Tabs from "$lib/components/ui/tabs/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import StatsChart from "$lib/components/charts/StatsChart.svelte";
	import Sparkline from "$lib/components/charts/Sparkline.svelte";
	import DateRangePicker from "$lib/components/date-range-picker.svelte";
	import { getDrives, getStatsDrive, getStatsVolume, getStatsApplication, getVolumes, getApplications, getStatsMigration } from "$lib/api";
	import { AreaChart, HardDrive, Layers, Box, History } from "@lucide/svelte";
	import type { MigrationStats } from "$lib/types";
	import { rangeDays, toISO, dateRange, type RangeKey } from "$lib/utils/dates";
	import { formatBytes } from "$lib/utils/format";

	type TabKey = "overview" | "volumes" | "drives" | "applications";

	let tab = $state<TabKey>((($page.url.searchParams.get("tab") as TabKey) || "overview"));

	// Overview tab state
	let overviewRange = $state<RangeKey>("7d");
	let driveCharts = $state<{ mountpoint: string; device: string; data: { date: Date; value: number }[] }[]>([]);
	let volumeCharts = $state<{ name: string; app: string; total: number; data: { date: Date; value: number }[] }[]>([]);
	let appCharts = $state<{ name: string; data: { date: Date; value: number }[] }[]>([]);
	let migrationStats = $state<MigrationStats | null>(null);
	let drivesLoading = $state(true);
	let volumesLoading = $state(true);
	let appsLoading = $state(true);
	let migrationsLoading = $state(true);
	let overviewAbort: AbortController | null = null;

	async function loadOverview() {
		overviewAbort?.abort();
		const ac = new AbortController();
		overviewAbort = ac;
		const signal = ac.signal;

		const from = toISO(rangeDays(overviewRange));

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

	function goToDetail(nextTab: "volumes" | "drives" | "applications", param: string, value: string) {
		const url = new URL(window.location.href);
		url.searchParams.set("tab", nextTab);
		url.searchParams.set(param, value);
		goto(url.pathname + url.search, { noScroll: true, keepFocus: true });
	}

	// Detail tabs (volumes / drives / applications) share the same shape:
	// a date-range picker, a small stat summary, and one StatsChart, keyed
	// by a query param instead of a route param.
	let detailName = $derived($page.url.searchParams.get("name") ?? "");
	let detailMountpoint = $derived($page.url.searchParams.get("mountpoint") ?? "");
	let detailRange = $state<RangeKey>("7d");

	let volumeData = $state<{ date: Date; value: number }[]>([]);
	let volumeCurrentSize = $state(0);
	let volumeLoading = $state(true);
	let volumeAbort: AbortController | null = null;

	async function loadVolumeDetail() {
		if (!detailName) return;
		volumeAbort?.abort();
		const ac = new AbortController();
		volumeAbort = ac;
		volumeLoading = volumeData.length === 0;
		try {
			const rows = await getStatsVolume(detailName, toISO(rangeDays(detailRange)), undefined, ac.signal);
			if (ac.signal.aborted) return;
			volumeData = rows.map(r => ({ date: new Date(r.captured_at), value: r.total_bytes }));
			if (rows.length > 0) volumeCurrentSize = rows[rows.length - 1].total_bytes;
		} catch {
			// aborted requests are expected, ignore
		} finally {
			if (!ac.signal.aborted) volumeLoading = false;
		}
	}

	let driveData = $state<{ date: Date; value: number }[]>([]);
	let driveUsed = $state(0);
	let driveFree = $state(0);
	let driveTotal = $state(0);
	let driveLoading = $state(true);
	let driveAbort: AbortController | null = null;

	async function loadDriveDetail() {
		if (!detailMountpoint) return;
		driveAbort?.abort();
		const ac = new AbortController();
		driveAbort = ac;
		driveLoading = driveData.length === 0;
		try {
			const rows = await getStatsDrive(detailMountpoint, toISO(rangeDays(detailRange)), undefined, ac.signal);
			if (ac.signal.aborted) return;
			driveData = rows.map(r => ({ date: new Date(r.captured_at), value: r.used_bytes }));
			if (rows.length > 0) {
				const last = rows[rows.length - 1];
				driveUsed = last.used_bytes;
				driveFree = last.free_bytes;
				driveTotal = last.total_bytes;
			}
		} catch {
			// aborted requests are expected, ignore
		} finally {
			if (!ac.signal.aborted) driveLoading = false;
		}
	}

	let appData = $state<{ date: Date; value: number }[]>([]);
	let appCurrentSize = $state(0);
	let appLoading = $state(true);
	let appAbort: AbortController | null = null;

	async function loadAppDetail() {
		if (!detailName) return;
		appAbort?.abort();
		const ac = new AbortController();
		appAbort = ac;
		appLoading = appData.length === 0;
		try {
			const rows = await getStatsApplication(detailName, toISO(rangeDays(detailRange)), undefined, ac.signal);
			if (ac.signal.aborted) return;
			appData = rows.map(r => ({ date: new Date(r.captured_at), value: r.total_bytes ?? 0 }));
			if (rows.length > 0) appCurrentSize = rows[rows.length - 1].total_bytes ?? 0;
		} catch {
			// aborted requests are expected, ignore
		} finally {
			if (!ac.signal.aborted) appLoading = false;
		}
	}

	function selectTab(next: TabKey) {
		tab = next;
		const url = new URL(window.location.href);
		url.searchParams.set("tab", next);
		goto(url.pathname + url.search, { noScroll: true, keepFocus: true });
	}

	$effect(() => {
		if (tab === "volumes" && detailName) loadVolumeDetail();
	});
	$effect(() => {
		if (tab === "drives" && detailMountpoint) loadDriveDetail();
	});
	$effect(() => {
		if (tab === "applications" && detailName) loadAppDetail();
	});

	onMount(() => {
		if (tab === "overview") loadOverview();
	});

	onDestroy(() => {
		overviewAbort?.abort();
		volumeAbort?.abort();
		driveAbort?.abort();
		appAbort?.abort();
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
</script>

<div class="space-y-6">
	<div>
		<h1 class="text-2xl font-bold tracking-tight">{$t("stats.title")}</h1>
		<p class="text-muted-foreground">{$t("stats.description")}</p>
	</div>

	<Tabs.Root value={tab} onValueChange={(v) => selectTab(v as TabKey)}>
		<Tabs.List>
			<Tabs.Trigger value="overview">{$t("stats.tabs.overview")}</Tabs.Trigger>
			<Tabs.Trigger value="volumes" disabled={!detailName}>{$t("stats.tabs.volumes")}</Tabs.Trigger>
			<Tabs.Trigger value="drives" disabled={!detailMountpoint}>{$t("stats.tabs.drives")}</Tabs.Trigger>
			<Tabs.Trigger value="applications" disabled={!detailName}>{$t("stats.tabs.applications")}</Tabs.Trigger>
		</Tabs.List>

		<Tabs.Content value="overview" class="space-y-6 pt-4">
			<div class="flex items-start justify-end">
				<DateRangePicker bind:value={overviewRange} onchange={loadOverview} />
			</div>

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
							<button class="block text-left" onclick={() => goToDetail("drives", "mountpoint", chart.mountpoint)}>
								<StatsChart
									title={chart.mountpoint}
									description={chart.device}
									data={chart.data}
									height="180px"
									xDomain={dateRange(overviewRange)}
									formatX={(v: Date) => v.toLocaleDateString(undefined, { month: "short", day: "numeric" })}
									formatY={(v: number) => formatBytes(v)}
								/>
							</button>
						{/each}
					{/if}
				</div>
			</div>

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
							<button class="block text-left w-full" onclick={() => goToDetail("volumes", "name", chart.name)}>
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
							</button>
						{/each}
					{/if}
				</div>
			</div>

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
							<button class="block text-left" onclick={() => goToDetail("applications", "name", chart.name)}>
								<StatsChart
									title={chart.name}
									data={chart.data}
									height="180px"
									color="var(--chart-3)"
									xDomain={dateRange(overviewRange)}
									formatX={(v: Date) => v.toLocaleDateString(undefined, { month: "short", day: "numeric" })}
									formatY={(v: number) => formatBytes(v)}
								/>
							</button>
						{/each}
					{/if}
				</div>
			</div>
		</Tabs.Content>

		<Tabs.Content value="volumes" class="space-y-6 pt-4">
			{#if !detailName}
				<p class="text-muted-foreground">{$t("stats.selectFromOverview")}</p>
			{:else}
				<div class="flex items-start justify-between">
					<h2 class="text-xl font-semibold">{detailName}</h2>
					<DateRangePicker bind:value={detailRange} onchange={loadVolumeDetail} />
				</div>
				{#if volumeLoading && volumeData.length === 0}
					<div class="rounded-lg border bg-card p-4">
						<Skeleton class="h-4 w-24 mb-2" />
						<Skeleton class="h-8 w-32" />
					</div>
				{:else if volumeCurrentSize > 0}
					<div class="rounded-lg border bg-card p-4">
						<p class="text-sm text-muted-foreground">{$t("stats.storage")}</p>
						<p class="text-2xl font-bold">{formatBytes(volumeCurrentSize)}</p>
					</div>
				{/if}
				<StatsChart
					title={detailName}
					data={volumeData}
					height="350px"
					color="var(--chart-2)"
					xDomain={dateRange(detailRange)}
					formatX={(v: Date) => v.toLocaleDateString(undefined, { month: "short", day: "numeric" })}
					formatY={(v: number) => formatBytes(v)}
				/>
			{/if}
		</Tabs.Content>

		<Tabs.Content value="drives" class="space-y-6 pt-4">
			{#if !detailMountpoint}
				<p class="text-muted-foreground">{$t("stats.selectFromOverview")}</p>
			{:else}
				<div class="flex items-start justify-between">
					<h2 class="text-xl font-semibold">{detailMountpoint}</h2>
					<DateRangePicker bind:value={detailRange} onchange={loadDriveDetail} />
				</div>
				{#if driveLoading && driveData.length === 0}
					<div class="grid gap-4 sm:grid-cols-3">
						{#each [1, 2, 3] as _}
							<div class="rounded-lg border bg-card p-4">
								<Skeleton class="h-4 w-16 mb-2" />
								<Skeleton class="h-8 w-24" />
							</div>
						{/each}
					</div>
				{:else if driveTotal > 0}
					<div class="grid gap-4 sm:grid-cols-3">
						<div class="rounded-lg border bg-card p-4">
							<p class="text-sm text-muted-foreground">{$t("drives.table.total")}</p>
							<p class="text-2xl font-bold">{formatBytes(driveTotal)}</p>
						</div>
						<div class="rounded-lg border bg-card p-4">
							<p class="text-sm text-muted-foreground">{$t("drives.table.free")}</p>
							<p class="text-2xl font-bold">{formatBytes(driveFree)}</p>
						</div>
						<div class="rounded-lg border bg-card p-4">
							<p class="text-sm text-muted-foreground">{$t("drives.table.usage")}</p>
							<p class="text-2xl font-bold">{driveTotal > 0 ? ((driveUsed / driveTotal) * 100).toFixed(1) : "0"}%</p>
						</div>
					</div>
				{/if}
				<StatsChart
					title={detailMountpoint}
					data={driveData}
					height="350px"
					xDomain={dateRange(detailRange)}
					formatX={(v: Date) => v.toLocaleDateString(undefined, { month: "short", day: "numeric" })}
					formatY={(v: number) => formatBytes(v)}
				/>
			{/if}
		</Tabs.Content>

		<Tabs.Content value="applications" class="space-y-6 pt-4">
			{#if !detailName}
				<p class="text-muted-foreground">{$t("stats.selectFromOverview")}</p>
			{:else}
				<div class="flex items-start justify-between">
					<h2 class="text-xl font-semibold">{detailName}</h2>
					<DateRangePicker bind:value={detailRange} onchange={loadAppDetail} />
				</div>
				{#if appLoading && appData.length === 0}
					<div class="rounded-lg border bg-card p-4">
						<Skeleton class="h-4 w-24 mb-2" />
						<Skeleton class="h-8 w-32" />
					</div>
				{:else if appCurrentSize > 0}
					<div class="rounded-lg border bg-card p-4">
						<p class="text-sm text-muted-foreground">{$t("stats.storage")}</p>
						<p class="text-2xl font-bold">{formatBytes(appCurrentSize)}</p>
					</div>
				{/if}
				<StatsChart
					title={detailName}
					data={appData}
					height="350px"
					color="var(--chart-3)"
					xDomain={dateRange(detailRange)}
					formatX={(v: Date) => v.toLocaleDateString(undefined, { month: "short", day: "numeric" })}
					formatY={(v: number) => formatBytes(v)}
				/>
			{/if}
		</Tabs.Content>
	</Tabs.Root>
</div>
