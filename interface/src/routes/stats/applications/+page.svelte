<script lang="ts">
	import { onMount } from "svelte";
	import { page } from "$app/stores";
	import { t } from "$lib/i18n";
	import StatsChart from "$lib/components/charts/StatsChart.svelte";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import { getStatsApplication } from "$lib/api";

	type RangeKey = "7d" | "30d" | "90d" | "all";
	let selectedRange = $state<RangeKey>("7d");
	let data = $state<{ date: Date; value: number }[]>([]);
	let currentSize = $state(0);
	let loading = $state(true);

	let abortController: AbortController | null = null;

	const name = $derived($page.url.searchParams.get("name") ?? "");

	function rangeDays(key: RangeKey): number {
		if (key === "7d") return 7;
		if (key === "30d") return 30;
		if (key === "90d") return 90;
		return 365;
	}

	function toISO(daysAgo: number): string {
		const d = new Date();
		d.setDate(d.getDate() - daysAgo);
		return d.toISOString();
	}

	const dateRange = $derived.by(() => {
		const to = new Date();
		const from = new Date();
		from.setDate(from.getDate() - rangeDays(selectedRange));
		return [from, to] as [Date, Date];
	});

	async function load() {
		abortController?.abort();
		const ac = new AbortController();
		abortController = ac;
		const signal = ac.signal;

		loading = data.length === 0;
		const from = toISO(rangeDays(selectedRange));
		try {
			const rows = await getStatsApplication(name, from, undefined, signal);
			if (signal.aborted) return;
			data = rows.map(r => ({ date: new Date(r.captured_at), value: r.total_bytes ?? 0 }));
			if (rows.length > 0) {
				currentSize = rows[rows.length - 1].total_bytes ?? 0;
			}
		} catch {
			// aborted requests are expected, ignore
		} finally {
			if (!signal.aborted) loading = false;
		}
	}

	onMount(() => {
		load();
		return () => { abortController?.abort(); };
	});

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
			<h1 class="text-2xl font-bold tracking-tight">{name}</h1>
			<p class="text-muted-foreground">{$t("stats.storage")} {$t("stats.evolution")}</p>
		</div>
		<div class="flex">
			{#each ["7d", "30d", "90d", "all"] as range (range)}
				<button
					data-active={selectedRange === range}
					class="data-[active=true]:bg-muted/50 relative z-30 flex flex-1 flex-col justify-center gap-1 border-t px-4 py-3 text-start even:border-s sm:border-s sm:border-t-0 sm:px-6 sm:py-6"
					onclick={() => { selectedRange = range as RangeKey; load(); }}
				>
					<span class="text-xs text-muted-foreground">{$t(`stats.range.${range}`)}</span>
				</button>
			{/each}
		</div>
	</div>

	{#if loading && data.length === 0}
		<div class="rounded-lg border bg-card p-4">
			<Skeleton class="h-4 w-24 mb-2" />
			<Skeleton class="h-8 w-32" />
		</div>
	{:else if currentSize > 0}
		<div class="rounded-lg border bg-card p-4">
			<p class="text-sm text-muted-foreground">{$t("stats.storage")}</p>
			<p class="text-2xl font-bold">{formatBytes(currentSize)}</p>
		</div>
	{/if}

	<StatsChart
		title={name}
		data={data}
		height="350px"
		color="var(--chart-3)"
		xDomain={dateRange}
		formatX={(v: Date) => v.toLocaleDateString(undefined, { month: "short", day: "numeric" })}
		formatY={(v: number) => formatBytes(v)}
	/>
</div>
